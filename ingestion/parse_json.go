package ingestion

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// jsonObject is the canonical JSON record shape. Inputs/outputs are nested so a
// single object fully describes a transaction (and/or a network observation).
type jsonObject struct {
	TxID       string        `json:"txid"`
	Timestamp  string        `json:"timestamp"`
	Fee        *float64      `json:"fee"`
	FeeBTC     *float64      `json:"fee_btc"`
	FeeSats    *int64        `json:"fee_sats"`
	ScriptType string        `json:"script_type"`
	BaseSize   *int          `json:"base_size"`
	TotalSize  *int          `json:"total_size"`
	Weight     *int          `json:"weight"`
	VSize      *int          `json:"vsize"`
	Inputs     []jsonIO      `json:"inputs"`
	Outputs    []jsonIO      `json:"outputs"`
	Network    []jsonNetwork `json:"network_observations"`
	// Flat network fields (also accepted on the object itself).
	SrcIP   string `json:"src_ip"`
	SrcPort *int   `json:"src_port"`
	DstIP   string `json:"dst_ip"`
	DstPort *int   `json:"dst_port"`
	Country string `json:"country"`
	ASN     string `json:"asn"`
}

type jsonIO struct {
	Address string   `json:"address"`
	Amount  *float64 `json:"amount"`
	BTC     *float64 `json:"amount_btc"`
}

type jsonNetwork struct {
	SrcIP   string `json:"src_ip"`
	SrcPort *int   `json:"src_port"`
	DstIP   string `json:"dst_ip"`
	DstPort *int   `json:"dst_port"`
	Country string `json:"country"`
	ASN     string `json:"asn"`
}

// jsonParser streams either a top-level JSON array or NDJSON (one object/line).
// Each source object expands into multiple flat Rows (per input/output/obs).
type jsonParser struct {
	rc      io.ReadCloser
	dec     *json.Decoder
	ndjson  bool
	scanner *bufio.Scanner
	recNo   int
	pending []Row // expanded rows awaiting emission
	started bool
}

func newJSONParser(rc io.ReadCloser, ndjson bool) (*jsonParser, error) {
	p := &jsonParser{rc: rc, ndjson: ndjson}
	if ndjson {
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		p.scanner = sc
	} else {
		p.dec = json.NewDecoder(rc)
	}
	return p, nil
}

func (p *jsonParser) Next() (Row, bool, error) {
	if len(p.pending) > 0 {
		r := p.pending[0]
		p.pending = p.pending[1:]
		return r, true, nil
	}
	obj, ok, err := p.nextObject()
	if err != nil {
		return Row{}, true, err
	}
	if !ok {
		return Row{}, false, nil
	}
	p.pending = p.expand(obj)
	if len(p.pending) == 0 {
		return p.Next() // skip empties
	}
	r := p.pending[0]
	p.pending = p.pending[1:]
	return r, true, nil
}

func (p *jsonParser) nextObject() (jsonObject, bool, error) {
	var obj jsonObject
	if p.ndjson {
		for p.scanner.Scan() {
			line := strings.TrimSpace(p.scanner.Text())
			if line == "" {
				continue
			}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				return obj, true, fmt.Errorf("ndjson parse: %w", err)
			}
			return obj, true, nil
		}
		return obj, false, p.scanner.Err()
	}
	// Array streaming: read the opening '[' once, then objects until ']'.
	if !p.started {
		if _, err := p.dec.Token(); err != nil {
			return obj, false, fmt.Errorf("json open: %w", err)
		}
		p.started = true
	}
	if !p.dec.More() {
		return obj, false, nil
	}
	if err := p.dec.Decode(&obj); err != nil {
		return obj, true, fmt.Errorf("json decode: %w", err)
	}
	return obj, true, nil
}

func fnum(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}
func inum(i *int) string {
	if i == nil {
		return ""
	}
	return strconv.Itoa(*i)
}
func i64num(i *int64) string {
	if i == nil {
		return ""
	}
	return strconv.FormatInt(*i, 10)
}

// expand turns one object into flat Rows: a base tx row per input/output, and a
// row per network observation.
func (p *jsonParser) expand(o jsonObject) []Row {
	var rows []Row
	fee := o.Fee
	if fee == nil {
		fee = o.FeeBTC
	}
	base := func() Row {
		p.recNo++
		return Row{
			RecordNo:   p.recNo,
			TxID:       o.TxID,
			Timestamp:  o.Timestamp,
			Fee:        fnum(fee),
			FeeSats:    i64num(o.FeeSats),
			ScriptType: o.ScriptType,
			BaseSize:   inum(o.BaseSize),
			TotalSize:  inum(o.TotalSize),
			Weight:     inum(o.Weight),
			VSize:      inum(o.VSize),
		}
	}

	maxIO := len(o.Inputs)
	if len(o.Outputs) > maxIO {
		maxIO = len(o.Outputs)
	}
	for i := 0; i < maxIO; i++ {
		r := base()
		if i < len(o.Inputs) {
			r.InputAddress = o.Inputs[i].Address
			r.InputAmount = fnum(pick(o.Inputs[i].Amount, o.Inputs[i].BTC))
		}
		if i < len(o.Outputs) {
			r.OutputAddress = o.Outputs[i].Address
			r.OutputAmount = fnum(pick(o.Outputs[i].Amount, o.Outputs[i].BTC))
		}
		rows = append(rows, r)
	}
	if maxIO == 0 && o.TxID != "" {
		rows = append(rows, base()) // header-only tx (partial)
	}

	// Flat network fields on the object.
	if o.SrcIP != "" || o.DstIP != "" {
		rows = append(rows, p.netRow(o.TxID, o.Timestamp, o.SrcIP, o.SrcPort, o.DstIP, o.DstPort, o.Country, o.ASN))
	}
	for _, n := range o.Network {
		rows = append(rows, p.netRow(o.TxID, o.Timestamp, n.SrcIP, n.SrcPort, n.DstIP, n.DstPort, n.Country, n.ASN))
	}
	return rows
}

func (p *jsonParser) netRow(txid, ts, sip string, sp *int, dip string, dp *int, country, asn string) Row {
	p.recNo++
	return Row{
		RecordNo: p.recNo, TxID: txid, Timestamp: ts,
		SrcIP: sip, SrcPort: inum(sp), DstIP: dip, DstPort: inum(dp),
		Country: country, ASN: asn,
	}
}

func pick(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

func (p *jsonParser) Close() error { return p.rc.Close() }
