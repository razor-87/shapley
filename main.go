package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
)

const (
	limitSinkBuffer = 1
	limitWorkers    = 2
)

type sValues struct {
	values map[string]float64
	mu     sync.Mutex
}

func main() {
	start := time.Now()
	if err := run(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
	fmt.Printf("Measure time: %s\n", time.Since(start))
}

func run() error {
	f, err := os.Open("data.csv")
	if err != nil {
		return fmt.Errorf("failed to open csv file, %w", err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Printf("[WARN] closing file: %v", err)
		}
	}()

	records, err := prepare(f)
	if err != nil {
		return fmt.Errorf("failed to prepare data, %w", err)
	}

	channels, worths, err := handle(records)
	if err != nil {
		return fmt.Errorf("failed to handle data, %w", err)
	}

	sValues := shapley(channels, worths)
	if err != nil {
		return fmt.Errorf("failed to calculate Shapley values, %w", err)
	}

	var checkSum float64
	for channel, value := range sValues {
		checkSum += value
		fmt.Printf("Channel: %s, Shapley value: %f\n", channel, value)
	}
	if notEqualsOne(checkSum) {
		return fmt.Errorf("sum of Shapley values isn't equal to one, %v", checkSum)
	}

	return nil
}

func prepare(r io.Reader) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read csv file, %w", err)
	}

	return records, nil
}

func handle(records [][]string) (channels []string, worths map[string]float64, err error) {
	set := make(map[string]struct{})
	cValues := make(map[string]float64, len(records))
	for _, rec := range records {
		if l := len(rec); l < 2 {
			return nil, nil, fmt.Errorf("length of slice less 2, %d", l)
		}

		row := strings.Split(rec[0], ",")
		sort.Strings(row)
		coalition := strings.Join(row, " ")
		val, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert string to int, %w", err)
		}

		for _, source := range row {
			set[source] = struct{}{}
		}
		cValues[coalition] = val
	}

	channels = make([]string, 0, len(set))
	for channel := range set {
		channels = append(channels, channel)
	}
	sort.Strings(channels)

	worths = make(map[string]float64, len(records))
	for coalition := range cValues {
		for c := range cValues {
			if containsAll(coalition, c) {
				worths[coalition] += cValues[c]
			}
		}
	}

	return channels, worths, nil
}

func containsAll(coalition, c string) bool {
	for _, player := range strings.Fields(c) {
		if !strings.Contains(coalition, player) {
			return false
		}
	}

	return true
}

func shapley(channels []string, worths map[string]float64) map[string]float64 {
	n := len(channels)
	svs := &sValues{values: make(map[string]float64, n)}

	sets := make([]chan []int, n)
	for g := 0; g < n; g++ {
		sets[g] = make(chan []int, limitSinkBuffer)
	}

	go func() {
		makeSubsetsIdxs(n, sets)
	}()

	var wgg sync.WaitGroup
	wgg.Add(n)
	for i, channel := range channels {
		go func(i int, channel string) {
			defer wgg.Done()

			chW := make(chan float64, 1)
			var wg sync.WaitGroup
			wg.Add(limitWorkers)
			for l := 0; l < limitWorkers; l++ {
				go func() {
					defer wg.Done()

					for subsetIdxs := range sets[i] {
						if slices.Contains(subsetIdxs, i) {
							continue
						}

						S := make([]string, len(subsetIdxs))
						for ii := range subsetIdxs {
							S[ii] = channels[subsetIdxs[ii]]
						}
						k := len(S)
						A := strings.Join(S, " ")
						Si := S
						Si = append(Si, channel)
						sort.Strings(Si)
						Ai := strings.Join(Si, " ")

						nominator := factorial(k) * factorial(n-k-1)
						denominator := factorial(n)
						// Weight = |S|!(n-|S|-1)!/n!
						weight := nominator / denominator
						// Marginal contribution = v(S U {i})-v(S)
						contrib := worths[Ai] - worths[A]

						chW <- weight * contrib
					}
				}()
			}

			chSum := make(chan float64)
			go func() {
				var sum float64
				for sValue := range chW {
					sum += sValue
				}
				chSum <- sum
			}()

			wg.Wait()
			close(chW)

			s := <-chSum

			svs.mu.Lock()
			svs.values[channel] = s + worths[channel]/float64(n)
			svs.mu.Unlock()

		}(i, channel)
	}
	wgg.Wait()

	return svs.values
}

func makeSubsetsIdxs(n int, sets []chan []int) {
	max := (1 << n) - 1

	go func() {
		defer func() {
			for _, set := range sets {
				close(set)
			}
		}()

		for binNum := 1; binNum <= max; binNum++ { // if binNum := 1 that without a null set
			var subsetIdxs []int
			for i := 0; i < n; i++ {
				if (1<<i)&binNum > 0 {
					subsetIdxs = append(subsetIdxs, i)
				}
			}
			if subsetIdxs != nil {
				for _, set := range sets {
					set <- subsetIdxs
				}
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
	return math.Abs(f-1) > 1e-9
}
