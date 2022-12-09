package main

import (
	"bytes"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

const data = `
Google,0.18
Meta,0.04
Microsoft,0.08
Meta Google,0.1
Microsoft Google,0.26
Meta Microsoft,0.07
Meta Microsoft Google,0.27
`

func mockData() io.Reader {
	return strings.NewReader(strings.TrimSpace(data))
}

func mockRecords() [][]string {
	return [][]string{{"Google", "0.18"}, {"Meta", "0.04"}, {"Microsoft", "0.08"}, {"Meta Google", "0.1"}, {"Microsoft Google", "0.26"}, {"Meta Microsoft", "0.07"}, {"Meta Microsoft Google", "0.27"}}
}

func mockChannels() []string {
	return []string{"Google", "Meta", "Microsoft"}
}

func mockWorths() map[string]float64 {
	return map[string]float64{"Google": 0.18, "Google Meta": 0.32, "Google Meta Microsoft": 1, "Google Microsoft": 0.52, "Meta": 0.04, "Meta Microsoft": 0.19, "Microsoft": 0.08}
}

func mockReader() (io.Reader, int) {
	f, _ := os.ReadFile("data/N11")
	return bytes.NewReader(f), 11
}

func Test_prepare(t *testing.T) {
	type args struct {
		r io.Reader
		g int
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "simple",
			args: args{r: mockData(), g: 3},
			want: mockRecords(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prepare(tt.args.r, tt.args.g)
			if (err != nil) != tt.wantErr {
				t.Errorf("prepare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prepare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handle(t *testing.T) {
	type args struct {
		records [][]string
	}
	tests := []struct {
		name         string
		args         args
		wantChannels []string
		wantWorths   map[string]float64
		wantErr      bool
	}{
		{
			name:         "simple",
			args:         args{mockRecords()},
			wantChannels: mockChannels(),
			wantWorths:   mockWorths(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChannels, gotWorths, err := handle(tt.args.records)
			if (err != nil) != tt.wantErr {
				t.Errorf("handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotChannels, tt.wantChannels) {
				t.Errorf("handle() gotChannels = %v, want %v", gotChannels, tt.wantChannels)
			}
			for key, value := range gotWorths {
				if wantValue := tt.wantWorths[key]; math.Abs(wantValue-value) > 1e-9 {
					t.Errorf("wantValue = %v, gotValue = %v", wantValue, value)
				}
			}
		})
	}
}

func Test_shapley(t *testing.T) {
	type args struct {
		channels []string
		worths   map[string]float64
	}
	tests := []struct {
		name string
		args args
		want map[string]float64
	}{
		{
			name: "simple",
			args: args{channels: mockChannels(), worths: mockWorths()},
			want: map[string]float64{"Google": 0.45, "Meta": 0.215, "Microsoft": 0.335},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := shapley(tt.args.channels, tt.args.worths)
			for key, value := range got {
				if wantValue := tt.want[key]; math.Abs(wantValue-value) > 1e-9 {
					t.Errorf("wantValue = %v, gotValue = %v", wantValue, value)
				}
			}
			if notEqualsOne(got1) {
				t.Errorf("shapley() got1 = %v, want 1", got1)
			}
		})
	}
}

func BenchmarkPrepare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		prepare(mockReader())
	}
}

func BenchmarkHandle(b *testing.B) {
	records, _ := prepare(mockReader())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handle(records)
	}
}

func BenchmarkShapley(b *testing.B) {
	records, _ := prepare(mockReader())
	channels, worths, _ := handle(records)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shapley(channels, worths)
	}
}
