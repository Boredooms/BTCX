package ingestion

import (
	"encoding/xml"
	"io"
	"strconv"
)

// XML schema (canonical):
//
//	<dataset>
//	  <transaction txid="..." timestamp="..." fee="..." script_type="..."
//	               base_size=".." total_size=".." weight=".." vsize="..">
//	    <input address="..." amount="..."/>
//	    <output address="..." amount="..."/>
//	    <observation src_ip=".." src_port=".." dst_ip=".." dst_port=".."
//	                 country=".." asn=".."/>
//	  </transaction>
//	</dataset>
//
// Decoded token-by-token so large files never build a full DOM.
type xmlTransaction struct {
	TxID       string   `xml:"txid,attr"`
	Timestamp  string   `xml:"timestamp,attr"`
	Fee        string   `xml:"fee,attr"`
	FeeSats    string   `xml:"fee_sats,attr"`
	ScriptType string   `xml:"script_type,attr"`
	BaseSize   string   `xml:"base_size,attr"`
	TotalSize  string   `xml:"total_size,attr"`
	Weight     string   `xml:"weight,attr"`
	VSize      string   `xml:"vsize,attr"`
	Inputs     []xmlIO  `xml:"input"`
	Outputs    []xmlIO  `xml:"output"`
	Obs        []xmlObs `xml:"observation"`
}

type xmlIO struct {
	Address string `xml:"address,attr"`
	Amount  string `xml:"amount,attr"`
}

type xmlObs struct {
	SrcIP   string `xml:"src_ip,attr"`
	SrcPort string `xml:"src_port,attr"`
	DstIP   string `xml:"dst_ip,attr"`
	DstPort string `xml:"dst_port,attr"`
	Country string `xml:"country,attr"`
	ASN     string `xml:"asn,attr"`
}

type xmlParser struct {
	rc      io.ReadCloser
	dec     *xml.Decoder
	recNo   int
	pending []Row
}

func newXMLParser(rc io.ReadCloser) (*xmlParser, error) {
	return &xmlParser{rc: rc, dec: xml.NewDecoder(rc)}, nil
}

func (p *xmlParser) Next() (Row, bool, error) {
	if len(p.pending) > 0 {
		r := p.pending[0]
		p.pending = p.pending[1:]
		return r, true, nil
	}
	for {
		tok, err := p.dec.Token()
		if err == io.EOF {
			return Row{}, false, nil
		}
		if err != nil {
			return Row{}, true, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "transaction" {
			continue
		}
		var tx xmlTransaction
		if err := p.dec.DecodeElement(&tx, &se); err != nil {
			p.recNo++
			return Row{RecordNo: p.recNo, Raw: err.Error()}, true, err
		}
		p.pending = p.expand(tx)
		if len(p.pending) == 0 {
			continue
		}
		r := p.pending[0]
		p.pending = p.pending[1:]
		return r, true, nil
	}
}

func (p *xmlParser) expand(tx xmlTransaction) []Row {
	var rows []Row
	base := func() Row {
		p.recNo++
		return Row{
			RecordNo: p.recNo, TxID: tx.TxID, Timestamp: tx.Timestamp,
			Fee: tx.Fee, FeeSats: tx.FeeSats, ScriptType: tx.ScriptType,
			BaseSize: tx.BaseSize, TotalSize: tx.TotalSize,
			Weight: tx.Weight, VSize: tx.VSize,
		}
	}
	maxIO := len(tx.Inputs)
	if len(tx.Outputs) > maxIO {
		maxIO = len(tx.Outputs)
	}
	for i := 0; i < maxIO; i++ {
		r := base()
		if i < len(tx.Inputs) {
			r.InputAddress = tx.Inputs[i].Address
			r.InputAmount = tx.Inputs[i].Amount
		}
		if i < len(tx.Outputs) {
			r.OutputAddress = tx.Outputs[i].Address
			r.OutputAmount = tx.Outputs[i].Amount
		}
		rows = append(rows, r)
	}
	if maxIO == 0 && tx.TxID != "" {
		rows = append(rows, base())
	}
	for _, o := range tx.Obs {
		p.recNo++
		rows = append(rows, Row{
			RecordNo: p.recNo, TxID: tx.TxID, Timestamp: tx.Timestamp,
			SrcIP: o.SrcIP, SrcPort: o.SrcPort, DstIP: o.DstIP, DstPort: o.DstPort,
			Country: o.Country, ASN: o.ASN,
		})
	}
	return rows
}

func (p *xmlParser) Close() error { return p.rc.Close() }

// atoiSafe parses an int, returning 0 on error (used for optional attrs).
func atoiSafe(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}
