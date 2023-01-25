package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/bits"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const epsilon = 1e-9

var (
	cpuprofile   = flag.Bool("cpuprofile", false, "write cpu profile to cpu.prof")
	memprofile   = flag.Bool("memprofile", false, "write memory profile to mem.prof")
	blockprofile = flag.Bool("blockprofile", false, "write block profile to block.prof")
	tracing      = flag.Bool("trace", false, "write tracing the execution of a program to trace.out")
	genes        = flag.Int("genes", 9, "number of genes")
)

func main() {
	flag.Parse()
	var r func() error
	if noFlags := !(*cpuprofile || *memprofile || *blockprofile || *tracing); noFlags {
		r = run
	} else {
		r = runWithFlags
	}
	if err := r(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

func run() error {
	start := time.Now()
	sValues, err := calc()
	if err != nil {
		return err
	}
	elapsed := time.Since(start)

	genes := make([]string, 0, len(sValues))
	for gene := range sValues {
		genes = append(genes, gene)
	}
	sort.Slice(genes, func(i, j int) bool {
		return sValues[genes[i]] < sValues[genes[j]]
	})
	for _, gene := range genes {
		fmt.Printf("Gene: %s, Shapley value: %f\n", gene, sValues[gene])
	}
	fmt.Printf("Measure time: %s\n", elapsed)

	return nil
}

func runWithFlags() error {
	if *cpuprofile {
		f, err := os.Create("cpu.prof")
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}
	if *blockprofile {
		runtime.SetBlockProfileRate(1)
		f, err := os.Create("block.prof")
		if err != nil {
			return fmt.Errorf("could not create block profile: %w", err)
		}
		defer f.Close()
		defer func() {
			if err = pprof.Lookup("block").WriteTo(f, 0); err != nil {
				log.Printf("[WARN] write to file: %v", err)
			}
		}()
	}
	if *tracing {
		f, err := os.Create("trace.out")
		if err != nil {
			return fmt.Errorf("failed to create trace output file: %w", err)
		}
		defer f.Close()
		if err := trace.Start(f); err != nil {
			return fmt.Errorf("failed to start trace: %w", err)
		}
		defer trace.Stop()
	}

	if _, err := calc(); err != nil {
		return err
	}

	if *memprofile {
		f, err := os.Create("mem.prof")
		if err != nil {
			return fmt.Errorf("could not create memory profile: %w", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("could not write memory profile: %w", err)
		}
	}

	return nil
}

func calc() (map[string]float64, error) {
	f, err := os.Open("data/N" + strconv.Itoa(*genes))
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file, %w", err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Printf("[WARN] closing file: %v", err)
		}
	}()

	records, err := prepare(f, *genes)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare data, %w", err)
	}

	players, bitset, worths, err := handle(records)
	if err != nil {
		return nil, fmt.Errorf("failed to handle data, %w", err)
	}

	sValues, checkSum := shapley(players, bitset, worths)
	if notEqualsOne(checkSum) {
		return nil, fmt.Errorf("sum of Shapley values isn't equal to one, %v", checkSum)
	}

	return sValues, nil
}

func prepare(r io.Reader, g int) ([][]string, error) {
	records := make([][]string, 0, (1<<g)-1)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		record := strings.Split(sc.Text(), ",")
		if l := len(record); l < 2 {
			return nil, fmt.Errorf("length of row less 2, %d", l)
		}
		records = append(records, record)
	}

	return records, nil
}

func handle(records [][]string) (players []string, bitset []uint16, worths map[uint16]float64, err error) {
	lenRecords := len(records)
	players = strings.Fields(records[lenRecords-1][0])
	sort.Strings(players)

	lenPlayers := len(players)
	bitset = make([]uint16, lenPlayers)
	mapBits := make(map[string]uint16, lenPlayers)
	var bit uint16
	for i, player := range players {
		bit = 1 << i
		bitset[i] = bit
		mapBits[player] = bit
	}

	cValues := make(map[uint16]float64, lenRecords)
	worths = make(map[uint16]float64, lenRecords)
	for _, rec := range records {
		vec := strings.Fields(rec[0])
		coalition := mapBits[vec[0]]
		for _, v := range vec[1:] {
			coalition |= mapBits[v]
		}

		var worth float64
		for bit, cValue := range cValues {
			if ^coalition&bit == 0 {
				worth += cValue
			}
		}

		cValue, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to convert string to int, %w", err)
		}
		cValues[coalition] = cValue
		worths[coalition] = worth + cValue
	}

	return players, bitset, worths, nil
}

func shapley(players []string, bitset []uint16, worths map[uint16]float64) (map[string]float64, float64) {
	n := len(players)
	vector := make([]float64, n)
	weight := makeWeight(n)

	powersets := make([]chan uint16, n)
	buffer := 1 << (n / 2)
	for ch := 0; ch < n; ch++ {
		powersets[ch] = make(chan uint16, buffer)
	}

	go func() {
		makeSubsetsIdxs(n, powersets)
	}()

	var wgg sync.WaitGroup
	wgg.Add(n)
	for i, bs := range bitset {
		vector[i] = worths[bs] / float64(n)

		go func(i int, bs uint16) {
			defer wgg.Done()

			var pSum float64
			for S := range powersets[i] {
				if S&bs != 0 {
					continue
				}

				k := bits.OnesCount16(S)
				Si := S | bs
				// Weight = |S|!(n-|S|-1)!/n!
				w := weight(k)
				// Marginal contribution = v(S U {i})-v(S)
				contrib := worths[Si] - worths[S]

				pSum += w * contrib
			}

			vector[i] += pSum
		}(i, bs)
	}
	wgg.Wait()

	var vSum float64
	sValues := make(map[string]float64, n)
	for i, value := range vector {
		vSum += value
		sValues[players[i]] = value
	}

	return sValues, vSum
}

func makeSubsetsIdxs(n int, powersets []chan uint16) {
	max := (1 << n) - 1

	go func() {
		defer func() {
			for _, powerset := range powersets {
				close(powerset)
			}
		}()

		for set := 1; set <= max; set++ { // if set := 1 that without a null set
			for _, powerset := range powersets {
				powerset <- uint16(set)
			}
		}
	}()
}

func notEqualsOne(f float64) bool {
	return math.Abs(f-1) > epsilon
}

func makeWeight(n int) func(k int) float64 {
	wsn := weights[n]
	return func(k int) float64 { return wsn[k] }
}
