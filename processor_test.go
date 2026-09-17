package ynabber

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type testProcessor struct {
	name    string
	process func(context.Context, []Transaction) ([]Transaction, error)
}

func (p *testProcessor) String() string { return p.name }
func (p *testProcessor) Process(ctx context.Context, batch []Transaction) ([]Transaction, error) {
	return p.process(ctx, batch)
}

func TestProcessorsRunInOrderBeforeFanOut(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			input := []Transaction{{ID: "first", Payee: "Shop"}, {ID: "second", Payee: "Cafe"}}
			reader := &mockOneShotReader{data: input}
			writers := []*mockWriter{{}, {}}
			var calls []string
			makeProcessor := func(name string) Processor {
				return &testProcessor{name: name, process: func(_ context.Context, batch []Transaction) ([]Transaction, error) {
					calls = append(calls, name)
					result := slices.Clone(batch)
					for i := range result {
						result[i].Payee = name + result[i].Payee
					}
					return result, nil
				}}
			}
			var options []Option
			if enabled {
				options = append(options, WithProcessors(makeProcessor("A"), makeProcessor("B")))
			}
			y, err := New([]Reader{reader}, []Writer{writers[0], writers[1]}, nil, options...)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := y.Run(ctx); err != nil {
				t.Fatal(err)
			}
			want := slices.Clone(input)
			if enabled {
				if !reflect.DeepEqual(calls, []string{"A", "B"}) {
					t.Fatalf("calls = %v", calls)
				}
				for i := range want {
					want[i].Payee = "BA" + want[i].Payee
				}
			}
			for _, w := range writers {
				if got := w.getBatches(); !reflect.DeepEqual(got, [][]Transaction{want}) {
					t.Fatalf("batches = %#v, want %#v", got, want)
				}
			}
			if input[0].Payee != "Shop" || input[1].Payee != "Cafe" {
				t.Fatal("reader input changed")
			}
		})
	}
}

func TestWithProcessorsCopiesSlice(t *testing.T) {
	original := &testProcessor{name: "original"}
	replacement := &testProcessor{name: "replacement"}
	processors := []Processor{original}
	option := WithProcessors(processors...)
	processors[0] = replacement
	newPipeline := func() *Ynabber {
		y, err := New([]Reader{&mockOneShotReader{}}, []Writer{&mockWriter{}}, nil, option, WithProcessors(replacement))
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	first, second := newPipeline(), newPipeline()
	if first.processors[0] != original || first.processors[1] != replacement {
		t.Fatal("processor options did not preserve order and ownership")
	}
	first.processors[0] = replacement
	if second.processors[0] != original {
		t.Fatal("pipelines share processor slices")
	}
}

func TestProcessorFailureCancelsPipelineWithoutDelivery(t *testing.T) {
	want := errors.New("processing failed")
	failing := &testProcessor{name: "broken", process: func(_ context.Context, batch []Transaction) ([]Transaction, error) { return batch, want }}
	writers := []*mockWriter{{}, {}}
	y, err := New([]Reader{&mockOneShotReader{data: []Transaction{{Payee: "Shop"}}}, waitingReader{}}, []Writer{writers[0], writers[1]}, nil, WithProcessors(failing))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = y.Run(ctx)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("error = %v", err)
	}
	for _, w := range writers {
		if len(w.getBatches()) != 0 {
			t.Fatal("failed batch delivered")
		}
	}
}

func TestProcessorCancellationStopsNextStage(t *testing.T) {
	for _, secondStage := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-writer", true: "between-processors"}[secondStage], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			first := &testProcessor{name: "cancel", process: func(_ context.Context, batch []Transaction) ([]Transaction, error) { cancel(); return batch, nil }}
			nextCalled := false
			next := &testProcessor{name: "next", process: func(_ context.Context, batch []Transaction) ([]Transaction, error) {
				nextCalled = true
				return batch, nil
			}}
			processors := []Processor{first}
			if secondStage {
				processors = append(processors, next)
			}
			writer := &mockWriter{}
			y, err := New([]Reader{&mockOneShotReader{data: []Transaction{{}}}}, []Writer{writer}, nil, WithProcessors(processors...))
			if err != nil {
				t.Fatal(err)
			}
			if err := y.Run(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v", err)
			}
			if nextCalled || len(writer.getBatches()) != 0 {
				t.Fatal("pipeline advanced after cancellation")
			}
		})
	}
}
