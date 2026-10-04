package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// csvParser streams a canonical CSV. The header maps column names to Row fields,
// so column order is flexible. Rows are read one at a time (never ReadAll).
type csvParser struct {
	rc     io.ReadCloser
	r      *csv.Reader
	colIdx map[string]int
	recNo  int
}

// canonical CSV column aliases -> Row field setter key.
var csvAliases = map[string]string{
	"txid": "txid", "tx_id": "txid", "transaction_id": "txid",
	"timestamp": "timestamp", "time": "timestamp", "ts": "timestamp",
	"fee": "fee", "fee_btc": "fee", "fee_sats": "fee_sats",
	"script_type": "script_type", "scripttype": "script_type",
	"base_size": "base_size", "base_size_vb": "base_size",
	"total_size": "total_size", "total_size_vb": "total_size", "size": "total_size",
	"weight": "weight", "weight_wu": "weight",
	"vsize": "vsize", "vsize_vb": "vsize",
	"input_address": "in_addr", "input_addresses": "in_addr",
	"input_amount": "in_amt", "input_amounts": "in_amt",
	"output_address": "out_addr", "output_addresses": "out_addr",
	"output_amount": "out_amt", "output_amounts": "out_amt",
	"src_ip": "src_ip", "source_ip": "src_ip",
	"src_port": "src_port", "source_port": "src_port",
	"dst_ip": "dst_ip", "dest_ip": "dst_ip", "destination_ip": "dst_ip",
	"dst_port": "dst_port", "dest_port": "dst_port",
	"country": "country", "asn": "asn",
}

func newCSVParser(rc io.ReadCloser) (*csvParser, error) {
	r := csv.NewReader(rc)
	r.FieldsPerRecord = -1 // tolerate ragged rows; validated downstream
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	colIdx := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if canon, ok := csvAliases[key]; ok {
			colIdx[canon] = i
		}
	}
	if _, ok := colIdx["txid"]; !ok {
		if _, nok := colIdx["src_ip"]; !nok {
			return nil, fmt.Errorf("csv header missing required txid or src_ip columns")
		}
	}
	return &csvParser{rc: rc, r: r, colIdx: colIdx}, nil
}

func (p *csvParser) get(rec []string, key string) string {
	if i, ok := p.colIdx[key]; ok && i < len(rec) {
		return strings.TrimSpace(rec[i])
	}
	return ""
}

func (p *csvParser) Next() (Row, bool, error) {
	rec, err := p.r.Read()
	if err == io.EOF {
		return Row{}, false, nil
	}
	if err != nil {
		// Return a row with Raw for diagnostics; importer records + continues.
		p.recNo++
		return Row{RecordNo: p.recNo, Raw: err.Error()}, true, err
	}
	p.recNo++
	row := Row{
		RecordNo:      p.recNo,
		TxID:          p.get(rec, "txid"),
		Timestamp:     p.get(rec, "timestamp"),
		Fee:           p.get(rec, "fee"),
		FeeSats:       p.get(rec, "fee_sats"),
		ScriptType:    p.get(rec, "script_type"),
		BaseSize:      p.get(rec, "base_size"),
		TotalSize:     p.get(rec, "total_size"),
		Weight:        p.get(rec, "weight"),
		VSize:         p.get(rec, "vsize"),
		InputAddress:  p.get(rec, "in_addr"),
		InputAmount:   p.get(rec, "in_amt"),
		OutputAddress: p.get(rec, "out_addr"),
		OutputAmount:  p.get(rec, "out_amt"),
		SrcIP:         p.get(rec, "src_ip"),
		SrcPort:       p.get(rec, "src_port"),
		DstIP:         p.get(rec, "dst_ip"),
		DstPort:       p.get(rec, "dst_port"),
		Country:       p.get(rec, "country"),
		ASN:           p.get(rec, "asn"),
		Raw:           strings.Join(rec, ","),
	}
	return row, true, nil
}

func (p *csvParser) Close() error { return p.rc.Close() }
