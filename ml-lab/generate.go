package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

var features = []string{
	"tx_count", "incoming_count", "outgoing_count", "incoming_volume", "outgoing_volume", "avg_amount", "amount_variance",
	"fee_mean", "tx_per_hour", "median_time_gap", "burstiness", "velocity", "degree", "fan_in", "fan_out",
	"counterparty_diversity", "graph_depth", "hop_count", "chain_length", "value_decay", "split_ratio", "merge_ratio",
	"obs_count", "unique_ip_count", "unique_asn_count",
}

var intFeature = map[string]bool{
	"tx_count": true, "incoming_count": true, "outgoing_count": true, "degree": true, "fan_in": true, "fan_out": true,
	"graph_depth": true, "hop_count": true, "chain_length": true, "obs_count": true, "unique_ip_count": true, "unique_asn_count": true,
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
func clampInt(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
func lognormal(r *rand.Rand, mu, sigma float64) float64 { return math.Exp(mu + sigma*r.NormFloat64()) }
func poissonApprox(r *rand.Rand, lambda float64) int {
	if lambda <= 0 {
		return 0
	} // Knuth for small/medium lambda; normal approximation for large.
	if lambda < 30 {
		l := math.Exp(-lambda)
		k := 0
		p := 1.0
		for p > l {
			k++
			p *= r.Float64()
		}
		return k - 1
	}
	x := lambda + math.Sqrt(lambda)*r.NormFloat64()
	if x < 0 {
		return 0
	}
	return int(math.Round(x))
}
func betaApprox(r *rand.Rand, a, b float64) float64 {
	x := math.Pow(r.ExpFloat64()/a, 1/a)
	y := math.Pow(r.ExpFloat64()/b, 1/b)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}
func dirichletEntropy(r *rand.Rand, k int) float64 {
	vals := make([]float64, k)
	sum := 0.0
	for i := range vals {
		vals[i] = r.ExpFloat64()
		sum += vals[i]
	}
	h := 0.0
	for _, v := range vals {
		p := v / sum
		h -= p * math.Log(math.Max(p, 1e-12))
	}
	return clamp(h/math.Log(float64(k)), 0, 1)
}
func id64(r *rand.Rand, counter int64) string {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[7-i] = byte(counter >> (8 * i))
	}
	h := sha256.Sum256(append(b, byte(r.Intn(256))))
	return hex.EncodeToString(h[:])
}
func fmt8(x float64) string { return fmt.Sprintf("%.8f", x) }

func walletFeatures(r *rand.Rand, scenario string, networkProb float64, entityFactor float64) []string {
	intensity := lognormal(r, 2.7, 0.85)
	mult := 1.0
	switch scenario {
	case "rapid_burst":
		mult = 5 + r.Float64()*10
	case "fanout_splitting":
		mult = 2 + r.Float64()*3
	case "fanin_consolidation":
		mult = 2 + r.Float64()*3
	case "micro_fragmentation":
		mult = 4 + r.Float64()*6
	case "velocity_spike":
		mult = 3 + r.Float64()*5
	case "large_value_jump":
		mult = 0.7 + r.Float64()*0.8
	case "deep_chain":
		mult = 1.5 + r.Float64()*2
	case "network_burst":
		mult = 1.5 + r.Float64()*2.5
	case "mixed_anomaly":
		mult = 3 + r.Float64()*4
	}
	tx := clampInt(int(math.Round(math.Max(2, intensity*mult))), 2, 2500)
	outShare := math.Pow(r.Float64(), 0.8)
	inShare := 1 - outShare
	fanOut := 1 + poissonApprox(r, 1.4+2.5*outShare+map[bool]float64{true: 1.2, false: 0}[scenario != "normal"])
	fanIn := 1 + poissonApprox(r, 1.2+2.0*inShare+map[bool]float64{true: 1.0, false: 0}[scenario != "normal"])
	if scenario == "fanout_splitting" {
		fanOut = 8 + r.Intn(32)
	}
	if scenario == "fanin_consolidation" {
		fanIn = 8 + r.Intn(32)
	}
	if scenario == "micro_fragmentation" {
		fanOut = 12 + r.Intn(68)
	}
	outgoing := clampInt(int(math.Round(float64(tx)*outShare)), 1, tx-1)
	incoming := tx - outgoing
	if incoming < 1 {
		incoming = 1
	}

	avg := lognormal(r, -2, 1.25) * (1 + 0.08*entityFactor)
	switch scenario {
	case "micro_fragmentation":
		avg *= 0.02 + 0.13*r.Float64()
	case "large_value_jump":
		avg *= 8 + 52*r.Float64()
	case "velocity_spike":
		avg *= 2 + 10*r.Float64()
	case "mixed_anomaly":
		avg *= 1.5 + 6.5*r.Float64()
	}
	avg = clamp(avg, 1e-6, 10000)
	cv := lognormal(r, -0.15, 0.6)
	if scenario == "micro_fragmentation" {
		cv *= 1.2 + 1.6*r.Float64()
	}
	variance := math.Pow(avg*cv, 2)
	outVol := float64(outgoing) * avg * lognormal(r, 0, 0.08)
	inVol := float64(incoming) * avg * lognormal(r, 0, 0.08)
	activeHours := clamp(lognormal(r, math.Log(24), 0.9), 0.05, 24*90)
	if scenario == "rapid_burst" {
		activeHours = 0.01 + 0.79*r.Float64()
	}
	if scenario == "velocity_spike" {
		activeHours = 0.03 + 1.97*r.Float64()
	}
	if scenario == "network_burst" {
		activeHours = 0.05 + 3.95*r.Float64()
	}
	txph := float64(tx) / activeHours
	total := inVol + outVol
	duration := math.Max(activeHours*3600, 1)
	velocity := total / duration
	medianGap := math.Max(activeHours*3600/float64(max(tx-1, 1)), 0.2)
	shape := 0.5 + 4*r.Float64()
	cvGap := 1 / math.Sqrt(shape)
	burst := (cvGap-1)/(cvGap+1) + 0.025*r.NormFloat64()
	burst = clamp(burst, -0.98, 0.98)
	split := clamp(1-dirichletMaxShare(r, 16), 0, 0.95)
	merge := clamp(1-dirichletMaxShare(r, 16), 0, 0.95)
	if scenario == "fanout_splitting" {
		split = 0.70 + 0.26*r.Float64()
	}
	if scenario == "fanin_consolidation" {
		merge = 0.70 + 0.26*r.Float64()
	}
	if scenario == "micro_fragmentation" {
		split = 0.80 + 0.195*r.Float64()
	}
	degree := max(1, fanIn+fanOut+poissonApprox(r, 2))
	diversity := dirichletEntropy(r, 16)
	depth := clampInt(poissonApprox(r, 2.5)+1, 1, 10)
	hops := clampInt(poissonApprox(r, 2)+1, 1, 10)
	chain := max(hops, poissonApprox(r, 3)+1)
	if scenario == "deep_chain" {
		hops = 6 + r.Intn(9)
		chain = hops + r.Intn(4)
	}
	decay := 0.70 + 0.25*betaApprox(r, 8.5, 1.5)
	if scenario == "deep_chain" {
		decay = 0.35 + 0.55*r.Float64()
	}
	hasNet := r.Float64() < networkProb
	obs := 0
	uip := 0
	uasn := 0
	if hasNet {
		obs = max(1, poissonApprox(r, math.Max(1, float64(tx)*0.35)))
		uip = min(obs, max(1, poissonApprox(r, float64(fanIn+fanOut)*0.12)))
		uasn = min(uip, max(1, poissonApprox(r, 1.1)))
	}
	if scenario == "network_burst" {
		obs = 25 + r.Intn(216)
		uip = 3 + r.Intn(22)
		uasn = 1 + min(uip-1, r.Intn(5))
	}
	vsize := clampInt(int(math.Round(lognormal(r, math.Log(180), 0.65))), 80, 8000)
	feerate := clamp(lognormal(r, math.Log(12), 0.75), 1, 2500)
	feeSats := math.Round(feerate * float64(vsize))
	feeBTC := feeSats / 100000000.0
	return []string{
		fmt.Sprintf("%d", tx), fmt.Sprintf("%d", incoming), fmt.Sprintf("%d", outgoing), fmt8(inVol), fmt8(outVol), fmt8(avg), fmt8(variance), fmt8(feeBTC), fmt8(txph), fmt8(medianGap), fmt8(burst), fmt8(velocity), fmt.Sprintf("%d", degree), fmt.Sprintf("%d", fanIn), fmt.Sprintf("%d", fanOut), fmt8(diversity), fmt.Sprintf("%d", depth), fmt.Sprintf("%d", hops), fmt.Sprintf("%d", chain), fmt8(decay), fmt8(split), fmt8(merge), fmt.Sprintf("%d", obs), fmt.Sprintf("%d", uip), fmt.Sprintf("%d", uasn),
	}
}
func dirichletMaxShare(r *rand.Rand, k int) float64 {
	sum := 0.0
	maxv := 0.0
	for i := 0; i < k; i++ {
		v := r.ExpFloat64()
		sum += v
		if v > maxv {
			maxv = v
		}
	}
	return maxv / sum
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeAnomaly(path string, rows int, seed int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 4<<20)
	defer bw.Flush()
	w := csv.NewWriter(bw)
	header := append([]string{"subject_id", "split", "scenario", "ground_truth_anomaly"}, features...)
	if err = w.Write(header); err != nil {
		return err
	}
	r := rand.New(rand.NewSource(seed))
	scenarios := []string{"rapid_burst", "fanout_splitting", "fanin_consolidation", "micro_fragmentation", "velocity_spike", "large_value_jump", "deep_chain", "network_burst", "mixed_anomaly"}
	for i := 0; i < rows; i++ {
		split := "train"
		if i%10 == 7 {
			split = "validation"
		}
		if i%10 >= 8 {
			split = "test"
		}
		sc := "normal"
		label := 0
		if split != "train" && r.Float64() < 0.52 {
			sc = scenarios[r.Intn(len(scenarios))]
			label = 1
		}
		vals := walletFeatures(r, sc, 0.68, 0)
		row := make([]string, 0, 4+len(vals))
		row = append(row, fmt.Sprintf("W%012d", i), split, sc, fmt.Sprintf("%d", label))
		row = append(row, vals...)
		if err = w.Write(row); err != nil {
			return err
		}
		if (i+1)%200000 == 0 {
			fmt.Printf("[anomaly] %d/%d\n", i+1, rows)
		}
	}
	w.Flush()
	return w.Error()
}

type EntityLatent struct {
	typ    string
	factor float64
	sc     string
}

func entityGroup(r *rand.Rand) (int, EntityLatent) {
	x := lognormal(r, 0.25, 0.9)
	size := clampInt(int(math.Round(x)), 1, 24)
	p := r.Float64()
	typ := "ordinary"
	switch {
	case p < .12:
		typ = "merchant"
	case p < .22:
		typ = "service"
	case p < .31:
		typ = "exchange_like"
	case p < .37:
		typ = "miner_like"
	case p < .43:
		typ = "custodial_like"
	}
	sc := "normal"
	if typ == "service" {
		sc = "rapid_burst"
	}
	if typ == "exchange_like" {
		sc = "fanin_consolidation"
	}
	if typ == "miner_like" {
		sc = "fanout_splitting"
	}
	return size, EntityLatent{typ, r.NormFloat64(), sc}
}
func writeEntity(path string, rows int, seed int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 4<<20)
	defer bw.Flush()
	w := csv.NewWriter(bw)
	hdr := []string{"wallet_id", "split", "true_entity_id", "entity_type", "cluster_size", "common_input_link_score", "same_entity_ground_truth", "relationship_basis"}
	hdr = append(hdr, features...)
	if err = w.Write(hdr); err != nil {
		return err
	}
	r := rand.New(rand.NewSource(seed))
	row := 0
	entityID := 0
	for row < rows {
		size, latent := entityGroup(r)
		if row+size > rows {
			size = rows - row
		}
		for j := 0; j < size; j++ {
			split := "fit"
			if row%10 == 7 {
				split = "validation"
			}
			if row%10 >= 8 {
				split = "test"
			}
			vals := walletFeatures(r, latent.sc, 0.52, latent.factor) // correlated perturbation of a few behavioral dimensions
			// Adjust selected values to preserve within-entity similarity.
			vals[5] = fmt8(parseFloat(vals[5]) * math.Exp(0.10*latent.factor+0.05*r.NormFloat64()))
			vals[8] = fmt8(parseFloat(vals[8]) * math.Exp(0.12*latent.factor+0.05*r.NormFloat64()))
			common := clamp(0.03+0.82*boolFloat(size > 1)+0.08*math.Tanh(latent.factor)+0.045*r.NormFloat64(), 0, 0.995)
			gt := 0
			basis := "singleton/no_link"
			if size > 1 {
				gt = 1
				basis = "shared_behavior+common_input_candidate"
			}
			rowData := []string{fmt.Sprintf("W%012d", row), split, fmt.Sprintf("E%010d", entityID), latent.typ, fmt.Sprintf("%d", size), fmt.Sprintf("%.4f", common), fmt.Sprintf("%d", gt), basis}
			rowData = append(rowData, vals...)
			if err = w.Write(rowData); err != nil {
				return err
			}
			row++
			if row%200000 == 0 {
				fmt.Printf("[entity] %d/%d\n", row, rows)
			}
		}
		entityID++
	}
	w.Flush()
	return w.Error()
}
func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
func parseFloat(s string) float64 { var x float64; fmt.Sscanf(s, "%f", &x); return x }

func writeFlow(path string, rows int, seed int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 4<<20)
	defer bw.Flush()
	w := csv.NewWriter(bw)
	hdr := []string{"flow_id", "split", "subject_id", "pattern_label", "suspicious_ground_truth", "input_count", "output_count", "participant_count", "hop_count", "chain_length", "value_decay", "split_ratio", "merge_ratio", "fan_in", "fan_out", "median_time_gap", "burstiness", "velocity", "total_volume_btc", "amount_entropy", "base_size_vb", "total_size_vb", "weight_wu", "vsize_vb", "feerate_sat_vb", "input_value_sats", "output_value_sats", "fee_sats"}
	if err = w.Write(hdr); err != nil {
		return err
	}
	r := rand.New(rand.NewSource(seed))
	patterns := []string{"normal", "peeling_chain", "mixing_like", "rapid_flow", "fan_out", "fan_in", "layered_chain", "burst_consolidation"}
	weights := []float64{.56, .12, .10, .07, .06, .04, .03, .02}
	pick := func() string {
		x := r.Float64()
		s := 0.0
		for i, p := range weights {
			s += p
			if x < s {
				return patterns[i]
			}
		}
		return "normal"
	}
	for i := 0; i < rows; i++ {
		p := pick()
		inC := 1 + poissonApprox(r, 2.2)
		outC := 1 + poissonApprox(r, 2.0)
		hops := 1 + poissonApprox(r, 2.2)
		chain := max(hops, 1+poissonApprox(r, 3))
		fin := inC + poissonApprox(r, 1)
		fout := outC + poissonApprox(r, 1)
		gap := lognormal(r, math.Log(300), 1.1)
		shape := .7 + 3*r.Float64()
		cv := 1 / math.Sqrt(shape)
		burst := (cv-1)/(cv+1) + .03*r.NormFloat64()
		burst = clamp(burst, -.99, .99)
		total := lognormal(r, -.2, 1.35)
		decay := .65 + .30*betaApprox(r, 8, 2)
		split := clamp(1-dirichletMaxShare(r, 16), 0, .98)
		merge := clamp(1-dirichletMaxShare(r, 16), 0, .98)
		velocity := total / math.Max(gap*float64(chain), 1)
		entropy := dirichletEntropy(r, 16)
		participants := max(2, fin+fout+poissonApprox(r, 1))
		switch p {
		case "peeling_chain":
			hops = 4 + r.Intn(7)
			chain = hops + r.Intn(4)
			decay = .72 + .265*r.Float64()
			split = .08 + .32*r.Float64()
			merge = .05 + .30*r.Float64()
			gap = lognormal(r, math.Log(45), .65)
		case "mixing_like":
			inC = 6 + r.Intn(26)
			outC = 6 + r.Intn(26)
			fin = inC + r.Intn(8)
			fout = outC + r.Intn(8)
			split = .55 + .35*r.Float64()
			merge = .55 + .35*r.Float64()
			entropy = .75 + .24*r.Float64()
			decay = .94 + .059*r.Float64()
			participants = 12 + r.Intn(68)
		case "rapid_flow":
			hops = 2 + r.Intn(6)
			chain = hops + r.Intn(3)
			gap = lognormal(r, math.Log(5), .65)
			velocity *= 8 + 32*r.Float64()
			burst = .20 + .75*r.Float64()
		case "fan_out":
			outC = 8 + r.Intn(72)
			fout = 10 + r.Intn(90)
			split = .72 + .275*r.Float64()
			participants = fout + 1 + r.Intn(20)
		case "fan_in":
			inC = 8 + r.Intn(72)
			fin = 10 + r.Intn(90)
			merge = .72 + .275*r.Float64()
			participants = fin + 1 + r.Intn(20)
		case "layered_chain":
			hops = 5 + r.Intn(10)
			chain = hops + 1 + r.Intn(5)
			decay = .35 + .53*r.Float64()
			gap = lognormal(r, math.Log(60), .85)
		case "burst_consolidation":
			inC = 4 + r.Intn(26)
			fin = 6 + r.Intn(34)
			merge = .60 + .36*r.Float64()
			burst = .15 + .70*r.Float64()
			velocity *= 3 + 12*r.Float64()
		}
		// BIP141-grounded size model: weight = 3*base_size + total_size; vsize = ceil(weight/4).
		baseSize := clampInt(int(math.Round(lognormal(r, math.Log(150), .65))), 65, 7000)
		witnessBytes := clampInt(int(math.Round(lognormal(r, math.Log(55), .9))), 0, 4000)
		totalSize := baseSize + witnessBytes
		weight := 3*baseSize + totalSize
		vsize := int(math.Ceil(float64(weight) / 4.0))
		feerate := clamp(lognormal(r, math.Log(12), .8), 1, 3000)
		inputSats := int64(math.Max(100000, math.Round(total*100000000.0)))
		feeCap := int64(math.Max(1, math.Floor(float64(inputSats)*0.002)))
		feeSats := int64(math.Min(math.Round(float64(vsize)*feerate), float64(feeCap)))
		outputSats := inputSats - feeSats
		rowData := []string{id64(r, int64(i)), chooseSplit(i), fmt.Sprintf("W%012d", i%5000000), p, fmt.Sprintf("%d", boolInt(p != "normal")), fmt.Sprintf("%d", inC), fmt.Sprintf("%d", outC), fmt.Sprintf("%d", participants), fmt.Sprintf("%d", hops), fmt.Sprintf("%d", chain), fmt8(clamp(decay, 0.05, .9999)), fmt8(clamp(split, 0, .999)), fmt8(clamp(merge, 0, .999)), fmt.Sprintf("%d", fin), fmt.Sprintf("%d", fout), fmt8(math.Max(gap, .1)), fmt.Sprintf("%.6f", burst), fmt8(velocity), fmt8(total), fmt8(entropy), fmt.Sprintf("%d", baseSize), fmt.Sprintf("%d", totalSize), fmt.Sprintf("%d", weight), fmt.Sprintf("%d", vsize), fmt8(feerate), fmt.Sprintf("%d", inputSats), fmt.Sprintf("%d", outputSats), fmt.Sprintf("%d", feeSats)}
		if err = w.Write(rowData); err != nil {
			return err
		}
		if (i+1)%200000 == 0 {
			fmt.Printf("[flow] %d/%d\n", i+1, rows)
		}
	}
	w.Flush()
	return w.Error()
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func chooseSplit(i int) string {
	if i%10 < 7 {
		return "train"
	}
	if i%10 < 8 {
		return "validation"
	}
	return "test"
}

func writeJSON(path string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(path, b, 0644)
}

func main() {
	outDir := flag.String("out-dir", "/mnt/data/BCTX_ML_Datasets", "")
	anMB := flag.Float64("anomaly-mb", 180, "")
	entMB := flag.Float64("entity-mb", 180, "")
	flowMB := flag.Float64("flow-mb", 180, "")
	seed := flag.Int64("seed", 20261002, "")
	flag.Parse()
	os.MkdirAll(*outDir, 0755)
	// Row-size estimates from representative sample rows; counts are approximate targets, then actual size is reported.
	anRows := int(*anMB * 1024 * 1024 / 202)
	entRows := int(*entMB * 1024 * 1024 / 258)
	flowRows := int(*flowMB * 1024 * 1024 / 219)
	fmt.Printf("rows: anomaly=%d entity=%d flow=%d\n", anRows, entRows, flowRows)
	an := filepath.Join(*outDir, "anomaly_dataset.csv")
	en := filepath.Join(*outDir, "entity_dataset.csv")
	fl := filepath.Join(*outDir, "flow_dataset.csv")
	if err := writeAnomaly(an, anRows, *seed+1); err != nil {
		panic(err)
	}
	if err := writeEntity(en, entRows, *seed+2); err != nil {
		panic(err)
	}
	if err := writeFlow(fl, flowRows, *seed+3); err != nil {
		panic(err)
	}
	meta := map[string]any{"generator_version": "bctx-synth-v1", "seed": *seed, "target_mib_per_dataset": map[string]float64{"anomaly": *anMB, "entity": *entMB, "flow": *flowMB}, "rows": map[string]int{"anomaly": anRows, "entity": entRows, "flow": flowRows}, "feature_schema": "feature-schema-v1", "labels": "synthetic development/evaluation ground truth; not real-world illicitness", "network_ranges": "raw IPs are intentionally omitted from model CSVs; any future network fixtures should use documentation/private ranges", "constraints": "flow rows enforce integer-satoshi conservation: input_value_sats=output_value_sats+fee_sats; size rows enforce BIP141 weight/vsize relation", "formulas": map[string]string{"fee": "fee_sats=sum(input_sats)-sum(output_sats)", "weight": "weight=3*base_size+total_size", "vsize": "vsize=ceil(weight/4)", "burstiness": "B=(sigma-mu)/(sigma+mu)", "diversity": "H/log(k), H=-sum(p_i log p_i)", "split_ratio": "1-max(output_share_i)", "merge_ratio": "1-max(input_share_i)", "value_decay": "terminal_value/initial_value", "velocity": "gross_flow_volume/active_seconds"}}
	writeJSON(filepath.Join(*outDir, "dataset_manifest.json"), meta)
}
