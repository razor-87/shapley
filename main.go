package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
)

const epsilon = 1e-9

var (
	cpuprofile = flag.Bool("cpuprofile", false, "write cpu profile to cpu.prof")
	memprofile = flag.Bool("memprofile", false, "write memory profile to mem.prof")
	tracing    = flag.Bool("trace", false, "write tracing the execution of a program to trace.out")
	genes      = flag.Int("genes", 9, "number of genes")
)

func main() {
	flag.Parse()
	var r func() error
	if noFlags := !(*cpuprofile || *tracing || *memprofile); noFlags {
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

	players, worths, err := handle(records)
	if err != nil {
		return nil, fmt.Errorf("failed to handle data, %w", err)
	}

	sValues, checkSum := shapley(players, worths)
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

func handle(records [][]string) (players []string, worths map[string]float64, err error) {
	set := make(map[string]struct{})
	cValues := make(map[string]float64, len(records))
	for _, rec := range records {
		vec := strings.Fields(rec[0])
		sort.Strings(vec)
		coalition := strings.Join(vec, " ")
		val, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert string to int, %w", err)
		}

		for _, source := range vec {
			set[source] = struct{}{}
		}
		cValues[coalition] = val
	}

	players = make([]string, 0, len(set))
	for player := range set {
		players = append(players, player)
	}
	sort.Strings(players)

	worths = make(map[string]float64, len(records))
	for coalition := range cValues {
		for c := range cValues {
			if containsAll(coalition, c) {
				worths[coalition] += cValues[c]
			}
		}
	}

	return players, worths, nil
}

func containsAll(coalition, c string) bool {
	for _, player := range strings.Fields(c) {
		if !strings.Contains(coalition, player) {
			return false
		}
	}

	return true
}

func shapley(players []string, worths map[string]float64) (map[string]float64, float64) {
	n := len(players)
	vector := make([]float64, n)

	powersets := make([]chan []int, n)
	buffer := 1 << (n / 2)
	for ch := 0; ch < n; ch++ {
		powersets[ch] = make(chan []int, buffer)
	}

	go func() {
		makeSubsetsIdxs(n, powersets)
	}()

	var wgg sync.WaitGroup
	wgg.Add(n)
	for i, player := range players {
		go func(i int, player string) {
			defer wgg.Done()

			var pSum float64
			for idxs := range powersets[i] {
				if slices.Contains(idxs, i) {
					continue
				}

				S := make([]string, len(idxs))
				for ii := range idxs {
					S[ii] = players[idxs[ii]]
				}
				k := len(S)
				A := strings.Join(S, " ")
				Si := S
				Si = append(Si, player)
				sort.Strings(Si)
				Ai := strings.Join(Si, " ")

				nominator := factorial(k) * factorial(n-k-1)
				denominator := factorial(n)
				// Weight = |S|!(n-|S|-1)!/n!
				weight := nominator / denominator
				// Marginal contribution = v(S U {i})-v(S)
				contrib := worths[Ai] - worths[A]

				pSum += weight * contrib
			}

			vector[i] = pSum + worths[player]/float64(n)
		}(i, player)
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

func makeSubsetsIdxs(n int, powersets []chan []int) {
	max := (1 << n) - 1

	go func() {
		defer func() {
			for _, powerset := range powersets {
				close(powerset)
			}
		}()

		for bin := 1; bin <= max; bin++ { // if bin := 1 that without a null set
			var subsetIndexes []int
			for i := 0; i < n; i++ {
				if (1<<i)&bin > 0 {
					subsetIndexes = append(subsetIndexes, i)
				}
			}

			for _, powerset := range powersets {
				powerset <- subsetIndexes
			}
		}
	}()
}

func factorial(n int) float64 {
	if n >= upperLimit {
		panic(fmt.Errorf("factorials upper limit"))
	}
	return float64(factorials[n])
}

func notEqualsOne(f float64) bool {
	return math.Abs(f-1) > epsilon
}
