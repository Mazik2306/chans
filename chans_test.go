package chans

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
)

func TestBroadcast(t *testing.T) {
	t.Run("zero chans", func(t *testing.T) {
		in := make(chan int, 3)
		outs := []chan<- int{}
		done := make(chan struct{})

		go func() {
			defer close(done)
			Broadcast(t.Context(), outs, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		<-done
		if len(in) != 0 {
			t.Error("should drain the input")
		}
	})
	t.Run("one chan", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		outs := []chan<- int{out}

		go func() {
			defer close(out)
			Broadcast(t.Context(), outs, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		want := []int{11, 22, 33}
		var got []int
		for v := range out {
			got = append(got, v)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("out 1: got %v, want %v", got, want)
		}
	})
	t.Run("two chans", func(t *testing.T) {
		in := make(chan int, 3)
		out1 := make(chan int, 3)
		out2 := make(chan int, 3)
		outs := []chan<- int{out1, out2}

		go func() {
			Broadcast(t.Context(), outs, in)
			close(out1)
			close(out2)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		want := []int{11, 22, 33}
		var got1 []int
		for v := range out1 {
			got1 = append(got1, v)
		}
		if !reflect.DeepEqual(got1, want) {
			t.Errorf("out 1: got %v, want %v", got1, want)
		}

		var got2 []int
		for v := range out2 {
			got2 = append(got2, v)
		}
		if !reflect.DeepEqual(got2, want) {
			t.Errorf("out 2: got %v, want %v", got2, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		outs := []chan<- int{out}

		go func() {
			defer close(out)
			Broadcast(t.Context(), outs, in)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out1 := make(chan int, 2)
		out2 := make(chan int, 2)
		outs := []chan<- int{out1, out2}

		go func() {
			Broadcast(ctx, outs, in)
			close(out1)
			close(out2)
		}()

		in <- 11
		in <- 22
		<-out1
		<-out1
		<-out2
		<-out2
		cancel()

		in <- 33
		in <- 44
		close(in)

		for v := range out1 {
			t.Errorf("should not emit after cancel, got %v", v)
		}
		for v := range out2 {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}

func TestChunk(t *testing.T) {
	t.Run("size zero", func(t *testing.T) {
		in := make(chan int, 2)
		out := make(chan []int, 2)

		go func() {
			defer close(out)
			Chunk(t.Context(), out, in, 0)
		}()

		in <- 11
		in <- 22
		close(in)

		for range out {
			t.Errorf("should not emit anything when n=0")
		}
	})
	t.Run("size one", func(t *testing.T) {
		in := make(chan int, 2)
		out := make(chan []int, 2)

		go func() {
			defer close(out)
			Chunk(t.Context(), out, in, 1)
		}()

		in <- 11
		in <- 22
		close(in)

		var got [][]int
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]int{{11}, {22}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("size two", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan []int, 5)

		go func() {
			defer close(out)
			Chunk(t.Context(), out, in, 2)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		in <- 55
		close(in)

		var got [][]int
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]int{{11, 22}, {33, 44}, {55}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("size greater than input", func(t *testing.T) {
		in := make(chan int, 2)
		out := make(chan []int, 2)

		go func() {
			defer close(out)
			Chunk(t.Context(), out, in, 10)
		}()

		in <- 11
		in <- 22
		close(in)

		var got [][]int
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]int{{11, 22}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan []int)

		go func() {
			defer close(out)
			Chunk(t.Context(), out, in, 2)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out := make(chan []int, 4)

		go func() {
			defer close(out)
			Chunk(ctx, out, in, 2)
		}()

		in <- 11
		in <- 22
		<-out // [11 22]
		cancel()

		in <- 33
		in <- 44
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestChunkBy(t *testing.T) {
	t.Run("by key", func(t *testing.T) {
		in := make(chan string, 4)
		out := make(chan []string, 4)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			ChunkBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "bb"
		in <- "cc"
		in <- "ddd"
		close(in)

		var got [][]string
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]string{{"a"}, {"bb", "cc"}, {"ddd"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("same key", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan []string, 3)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			ChunkBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "b"
		in <- "c"
		close(in)

		var got [][]string
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]string{{"a", "b", "c"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("distinct keys", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan []string, 3)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			ChunkBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "bb"
		in <- "ccc"
		close(in)

		var got [][]string
		for chunk := range out {
			got = append(got, chunk)
		}

		want := [][]string{{"a"}, {"bb"}, {"ccc"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan string)
		out := make(chan []string)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			ChunkBy(t.Context(), out, in, key)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan string, 5)
		out := make(chan []string, 5)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			ChunkBy(ctx, out, in, key)
		}()

		in <- "a"
		in <- "b"
		in <- "aa"
		<-out
		cancel()

		in <- "bb"
		in <- "ccc"
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestCompact(t *testing.T) {
	t.Run("some duplicates", func(t *testing.T) {
		in := make(chan int, 8)
		out := make(chan int, 8)

		go func() {
			defer close(out)
			Compact(t.Context(), out, in)
		}()

		in <- 11
		in <- 11
		in <- 22
		in <- 22
		in <- 22
		in <- 33
		in <- 11
		in <- 11
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33, 11}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("no duplicates", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Compact(t.Context(), out, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("all duplicates", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Compact(t.Context(), out, in)
		}()

		in <- 11
		in <- 11
		in <- 11
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Compact(t.Context(), out, in)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out := make(chan int, 4)

		go func() {
			defer close(out)
			Compact(ctx, out, in)
		}()

		in <- 11
		in <- 11
		<-out
		cancel()

		in <- 22
		in <- 22
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestCompactBy(t *testing.T) {
	t.Run("some duplicates", func(t *testing.T) {
		in := make(chan string, 8)
		out := make(chan string, 8)
		eq := func(a, b string) bool { return len(a) == len(b) }

		go func() {
			defer close(out)
			CompactBy(t.Context(), out, in, eq)
		}()

		in <- "a"
		in <- "b"
		in <- "aa"
		in <- "bb"
		in <- "cc"
		in <- "aaa"
		in <- "d"
		in <- "e"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a", "aa", "aaa", "d"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("no duplicates", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan string, 3)
		eq := func(a, b string) bool { return len(a) == len(b) }

		go func() {
			defer close(out)
			CompactBy(t.Context(), out, in, eq)
		}()

		in <- "a"
		in <- "bb"
		in <- "ccc"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a", "bb", "ccc"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("all duplicates", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan string, 3)
		eq := func(a, b string) bool { return len(a) == len(b) }

		go func() {
			defer close(out)
			CompactBy(t.Context(), out, in, eq)
		}()

		in <- "a"
		in <- "b"
		in <- "c"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan string)
		out := make(chan string)
		eq := func(a, b string) bool { return len(a) == len(b) }

		go func() {
			defer close(out)
			CompactBy(t.Context(), out, in, eq)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan string, 4)
		out := make(chan string, 4)
		eq := func(a, b string) bool { return len(a) == len(b) }

		go func() {
			defer close(out)
			CompactBy(ctx, out, in, eq)
		}()

		in <- "a"
		in <- "b"
		<-out
		cancel()

		in <- "aa"
		in <- "bb"
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestConcat(t *testing.T) {
	t.Run("no chans", func(t *testing.T) {
		out := make(chan int, 2)
		go func() {
			Concat(t.Context(), out)
			close(out)
		}()
		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("one chan", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 5)

		go func() {
			defer close(out)
			Concat(t.Context(), out, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("multiple chans", func(t *testing.T) {
		in1 := make(chan int, 2)
		in2 := make(chan int, 2)
		in3 := make(chan int, 1)
		out := make(chan int, 5)

		go func() {
			defer close(out)
			Concat(t.Context(), out, in1, in2, in3)
		}()

		in1 <- 11
		in1 <- 22
		close(in1)
		in2 <- 33
		in2 <- 44
		close(in2)
		in3 <- 55
		close(in3)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33, 44, 55}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in1 := make(chan int)
		in2 := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Concat(t.Context(), out, in1, in2)
		}()

		close(in1)
		close(in2)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in1 := make(chan int, 2)
		in2 := make(chan int, 2)
		out := make(chan int, 4)

		go func() {
			Concat(ctx, out, in1, in2)
			close(out)
		}()

		in1 <- 11
		in1 <- 22
		close(in1)
		<-out
		<-out
		cancel()

		in2 <- 33
		in2 <- 44
		close(in2)

		for v := range out {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}

func TestDistinct(t *testing.T) {
	t.Run("some duplicates", func(t *testing.T) {
		in := make(chan int, 8)
		out := make(chan int, 8)

		go func() {
			defer close(out)
			Distinct(t.Context(), out, in)
		}()

		in <- 11
		in <- 11
		in <- 22
		in <- 22
		in <- 22
		in <- 33
		in <- 11
		in <- 11
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("no duplicates", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Distinct(t.Context(), out, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("all duplicates", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Distinct(t.Context(), out, in)
		}()

		in <- 11
		in <- 11
		in <- 11
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Distinct(t.Context(), out, in)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out := make(chan int, 4)

		go func() {
			defer close(out)
			Distinct(ctx, out, in)
		}()

		in <- 11
		in <- 11
		<-out
		cancel()

		in <- 22
		in <- 22
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestDistinctBy(t *testing.T) {
	t.Run("some duplicates", func(t *testing.T) {
		in := make(chan string, 8)
		out := make(chan string, 8)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			DistinctBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "b"
		in <- "aa"
		in <- "bb"
		in <- "cc"
		in <- "aaa"
		in <- "d"
		in <- "e"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a", "aa", "aaa"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("no duplicates", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan string, 3)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			DistinctBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "bb"
		in <- "ccc"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a", "bb", "ccc"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("all duplicates", func(t *testing.T) {
		in := make(chan string, 3)
		out := make(chan string, 3)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			DistinctBy(t.Context(), out, in, key)
		}()

		in <- "a"
		in <- "b"
		in <- "c"
		close(in)

		var got []string
		for v := range out {
			got = append(got, v)
		}

		want := []string{"a"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan string)
		out := make(chan string)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			DistinctBy(t.Context(), out, in, key)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan string, 4)
		out := make(chan string, 4)
		key := func(v string) int { return len(v) }

		go func() {
			defer close(out)
			DistinctBy(ctx, out, in, key)
		}()

		in <- "a"
		in <- "b"
		<-out
		cancel()

		in <- "aa"
		in <- "bb"
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestDrain(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		in := make(chan int, 3)
		in <- 11
		in <- 22
		in <- 33
		close(in)

		Drain(t.Context(), in)
		if len(in) != 0 {
			t.Errorf("got len(in) = %d, want 0", len(in))
		}
	})
	t.Run("empty", func(t *testing.T) {
		in := make(chan int)
		close(in)
		Drain(t.Context(), in)
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		done := make(chan struct{})

		go func() {
			close(done)
			Drain(ctx, in)
		}()

		in <- 11
		in <- 22
		cancel()
		in <- 33
		in <- 44
		close(in)

		<-done
		if len(in) == 0 {
			t.Errorf("should not drain after cancel")
		}
	})
}

func TestDrop(t *testing.T) {
	t.Run("n equals zero", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Drop(t.Context(), out, in, 0)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n equals one", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Drop(t.Context(), out, in, 1)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n less than input", func(t *testing.T) {
		in := make(chan int, 4)
		out := make(chan int, 4)

		go func() {
			defer close(out)
			Drop(t.Context(), out, in, 2)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{33, 44}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n greater than input", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Drop(t.Context(), out, in, 10)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		if len(got) != 0 {
			t.Errorf("got %v, want []", got)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Drop(t.Context(), out, in, 2)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Drop(ctx, out, in, 2)
		}()

		in <- 11
		cancel()

		in <- 22
		in <- 33
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestDropWhile(t *testing.T) {
	t.Run("drop some", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)
		pred := func(v int) bool { return v < 33 }

		go func() {
			defer close(out)
			DropWhile(t.Context(), out, in, pred)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 22
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{33, 22, 44}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("drop none", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) bool { return false }

		go func() {
			defer close(out)
			DropWhile(t.Context(), out, in, pred)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("drop all", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) bool { return true }

		go func() {
			defer close(out)
			DropWhile(t.Context(), out, in, pred)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		if len(got) != 0 {
			t.Errorf("got %v, want []", got)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		pred := func(v int) bool { return false }

		go func() {
			defer close(out)
			DropWhile(t.Context(), out, in, pred)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) bool { return v < 33 }

		go func() {
			defer close(out)
			DropWhile(ctx, out, in, pred)
		}()

		in <- 11
		cancel()
		in <- 22
		in <- 33
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestFilter(t *testing.T) {
	t.Run("select some", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)
		pred := func(v int) (bool, error) { return v > 22, nil }

		go func() {
			defer close(out)
			err := Filter(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 33
		in <- 22
		in <- 44
		in <- 55
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{33, 44, 55}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("select none", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return false, nil }

		go func() {
			defer close(out)
			err := Filter(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		if len(got) != 0 {
			t.Errorf("got %v, want []", got)
		}
	})
	t.Run("select all", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return true, nil }

		go func() {
			defer close(out)
			err := Filter(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		pred := func(v int) (bool, error) { return true, nil }

		go func() {
			defer close(out)
			err := Filter(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("stop early", func(t *testing.T) {
		errStop := errors.New("stop early")
		in := make(chan int, 4)
		out := make(chan int, 4)
		pred := func(v int) (bool, error) {
			if v == 33 {
				return false, errStop
			}
			return true, nil
		}

		go func() {
			defer close(out)
			err := Filter(t.Context(), out, in, pred)
			if !errors.Is(err, errStop) {
				t.Errorf("got err %v, want %v", err, errStop)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return v > 22, nil }

		go func() {
			defer close(out)
			err := Filter(ctx, out, in, pred)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("got err %v, want context.Canceled", err)
			}
		}()

		in <- 33
		<-out
		cancel()
		in <- 22
		in <- 55
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestFilterOut(t *testing.T) {
	t.Run("drop some", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)
		pred := func(v int) (bool, error) { return v < 33, nil }

		go func() {
			defer close(out)
			err := FilterOut(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 33
		in <- 22
		in <- 44
		in <- 55
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{33, 44, 55}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("drop all", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return true, nil }

		go func() {
			defer close(out)
			err := FilterOut(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		if len(got) != 0 {
			t.Errorf("got %v, want []", got)
		}
	})
	t.Run("drop none", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return false, nil }

		go func() {
			defer close(out)
			err := FilterOut(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		pred := func(v int) (bool, error) { return true, nil }

		go func() {
			defer close(out)
			err := FilterOut(t.Context(), out, in, pred)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("stop early", func(t *testing.T) {
		errStop := errors.New("stop early")
		in := make(chan int, 4)
		out := make(chan int, 4)
		pred := func(v int) (bool, error) {
			if v == 33 {
				return false, errStop
			}
			return false, nil
		}

		go func() {
			defer close(out)
			err := FilterOut(t.Context(), out, in, pred)
			if !errors.Is(err, errStop) {
				t.Errorf("got err %v, want %v", err, errStop)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}

		want := []int{11, 22}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		out := make(chan int, 3)
		pred := func(v int) (bool, error) { return v < 33, nil }

		go func() {
			defer close(out)
			err := FilterOut(ctx, out, in, pred)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("got err %v, want context.Canceled", err)
			}
		}()

		in <- 33
		<-out
		cancel()
		in <- 22
		in <- 55
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestFirst(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		want := 22
		in := make(chan int, 3)
		pred := func(v int) bool { return v == want }

		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(t.Context(), in, pred)
		if !ok {
			t.Errorf("got ok = false, want true")
		}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("no match", func(t *testing.T) {
		in := make(chan int, 3)
		pred := func(v int) bool { return v == 42 }

		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(t.Context(), in, pred)
		if ok {
			t.Errorf("got ok = true, want false")
		}
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
	t.Run("true pred", func(t *testing.T) {
		in := make(chan int, 3)
		pred := func(v int) bool { return true }

		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(t.Context(), in, pred)
		if !ok {
			t.Errorf("got ok = false, want true")
		}
		want := 11
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("false pred", func(t *testing.T) {
		in := make(chan int, 3)
		pred := func(v int) bool { return false }

		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(t.Context(), in, pred)
		if ok {
			t.Errorf("got ok = true, want false")
		}
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
	t.Run("nil pred", func(t *testing.T) {
		in := make(chan int, 3)

		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(t.Context(), in, nil)
		if !ok {
			t.Errorf("got ok = false, want true")
		}
		want := 11
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		pred := func(v int) bool { return true }

		close(in)

		got, ok := First(t.Context(), in, pred)
		if ok {
			t.Errorf("got ok = true, want false")
		}
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		pred := func(v int) bool { return true }

		cancel()
		in <- 11
		in <- 22
		in <- 33
		close(in)

		got, ok := First(ctx, in, pred)
		if ok {
			t.Errorf("got ok = true, want false")
		}
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
}

func TestFlatten(t *testing.T) {
	t.Run("flatten", func(t *testing.T) {
		in := make(chan []int, 4)
		out := make(chan int, 6)

		go func() {
			defer close(out)
			Flatten(t.Context(), out, in)
		}()

		in <- []int{11, 22}
		in <- []int{33}
		in <- []int{}
		in <- []int{44, 55, 66}
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33, 44, 55, 66}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan []int)
		out := make(chan int)

		go func() {
			defer close(out)
			Flatten(t.Context(), out, in)
		}()

		close(in)
		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("empty slices", func(t *testing.T) {
		in := make(chan []int, 2)
		out := make(chan int, 2)

		go func() {
			defer close(out)
			Flatten(t.Context(), out, in)
		}()

		in <- []int{}
		in <- []int{}
		close(in)

		for v := range out {
			t.Errorf("should not emit anything for empty slices, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan []int, 2)
		out := make(chan int, 2)

		go func() {
			defer close(out)
			Flatten(ctx, out, in)
		}()

		in <- []int{11, 22}
		<-out
		<-out
		cancel()
		in <- []int{33, 44}
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestMap(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		in := make(chan string, 4)
		out := make(chan int, 4)
		fn := func(v string) (int, error) { return len(v), nil }

		go func() {
			defer close(out)
			err := Map(t.Context(), out, in, fn)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		in <- "a"
		in <- "bb"
		in <- "ccc"
		in <- ""
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{1, 2, 3, 0}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		fn := func(v int) (int, error) { return v * 2, nil }

		go func() {
			defer close(out)
			err := Map(t.Context(), out, in, fn)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()

		close(in)
		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("stop early", func(t *testing.T) {
		errStop := errors.New("stop early")
		in := make(chan int, 4)
		out := make(chan int, 4)
		fn := func(v int) (int, error) {
			if v == 33 {
				return 0, errStop
			}
			return v, nil
		}

		go func() {
			defer close(out)
			err := Map(t.Context(), out, in, fn)
			if !errors.Is(err, errStop) {
				t.Errorf("got err %v, want %v", err, errStop)
			}
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 3)
		out := make(chan int, 3)
		fn := func(v int) (int, error) { return v * 2, nil }

		go func() {
			defer close(out)
			err := Map(ctx, out, in, fn)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("got err %v, want context.Canceled", err)
			}
		}()

		in <- 11
		<-out
		cancel()
		in <- 22
		in <- 33
		close(in)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestMerge(t *testing.T) {
	t.Run("one chan", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 5)

		in <- 11
		in <- 22
		in <- 33
		close(in)

		go func() {
			defer close(out)
			Merge(t.Context(), out, in)
		}()

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("two chans", func(t *testing.T) {
		in1 := make(chan int, 3)
		in2 := make(chan int, 2)
		out := make(chan int, 5)

		in1 <- 11
		in1 <- 22
		in1 <- 33
		close(in1)
		in2 <- 44
		in2 <- 55
		close(in2)

		go func() {
			defer close(out)
			Merge(t.Context(), out, in1, in2)
		}()

		var got []int
		for v := range out {
			got = append(got, v)
		}
		// Order is not guaranteed, so sort for comparison.
		want := []int{11, 22, 33, 44, 55}
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("five chans", func(t *testing.T) {
		in1 := make(chan int, 1)
		in2 := make(chan int, 1)
		in3 := make(chan int, 1)
		in4 := make(chan int, 1)
		in5 := make(chan int, 1)
		out := make(chan int, 5)

		in1 <- 55
		in2 <- 44
		in3 <- 33
		in4 <- 22
		in5 <- 11
		close(in1)
		close(in2)
		close(in3)
		close(in4)
		close(in5)

		go func() {
			defer close(out)
			Merge(t.Context(), out, in1, in3, in2, in4, in5)
		}()

		var got []int
		for v := range out {
			got = append(got, v)
		}
		// Order is not guaranteed, so sort for comparison.
		want := []int{11, 22, 33, 44, 55}
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in1 := make(chan int)
		in2 := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Merge(t.Context(), out, in1, in2)
		}()

		close(in1)
		close(in2)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in1 := make(chan int, 2)
		in2 := make(chan int, 2)
		out := make(chan int, 4)

		in1 <- 11
		in2 <- 22

		go func() {
			defer close(out)
			Merge(ctx, out, in1, in2)
		}()

		<-out
		<-out
		cancel()
		in1 <- 33
		in2 <- 44
		close(in1)
		close(in2)

		for v := range out {
			t.Errorf("should not emit anything after cancel, got %v", v)
		}
	})
}

func TestPartition(t *testing.T) {
	t.Run("mixed", func(t *testing.T) {
		in := make(chan int, 5)
		outTrue := make(chan int, 5)
		outFalse := make(chan int, 5)
		pred := func(v int) bool { return v%2 == 0 }

		go func() {
			Partition(t.Context(), outTrue, outFalse, in, pred)
			close(outTrue)
			close(outFalse)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		in <- 55
		close(in)

		var gotTrue []int
		for v := range outTrue {
			gotTrue = append(gotTrue, v)
		}
		wantTrue := []int{22, 44}
		if !reflect.DeepEqual(gotTrue, wantTrue) {
			t.Errorf("outTrue: got %v, want %v", gotTrue, wantTrue)
		}

		var gotFalse []int
		for v := range outFalse {
			gotFalse = append(gotFalse, v)
		}
		wantFalse := []int{11, 33, 55}
		if !reflect.DeepEqual(gotFalse, wantFalse) {
			t.Errorf("outFalse: got %v, want %v", gotFalse, wantFalse)
		}
	})
	t.Run("all true", func(t *testing.T) {
		in := make(chan int, 3)
		outTrue := make(chan int, 3)
		outFalse := make(chan int, 3)
		pred := func(v int) bool { return true }

		go func() {
			Partition(t.Context(), outTrue, outFalse, in, pred)
			close(outTrue)
			close(outFalse)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var gotTrue []int
		wantTrue := []int{11, 22, 33}
		for v := range outTrue {
			gotTrue = append(gotTrue, v)
		}
		if !reflect.DeepEqual(gotTrue, wantTrue) {
			t.Errorf("outTrue: got %v, want %v", gotTrue, wantTrue)
		}

		var gotFalse []int
		for v := range outFalse {
			gotFalse = append(gotFalse, v)
		}
		if len(gotFalse) != 0 {
			t.Errorf("outFalse: got %v, want empty", gotFalse)
		}
	})
	t.Run("all false", func(t *testing.T) {
		in := make(chan int, 3)
		outTrue := make(chan int, 3)
		outFalse := make(chan int, 3)
		pred := func(v int) bool { return false }

		go func() {
			Partition(t.Context(), outTrue, outFalse, in, pred)
			close(outTrue)
			close(outFalse)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var gotTrue []int
		for v := range outTrue {
			gotTrue = append(gotTrue, v)
		}
		if len(gotTrue) != 0 {
			t.Errorf("outTrue: got %v, want empty", gotTrue)
		}

		var gotFalse []int
		for v := range outFalse {
			gotFalse = append(gotFalse, v)
		}
		wantFalse := []int{11, 22, 33}
		if !reflect.DeepEqual(gotFalse, wantFalse) {
			t.Errorf("outFalse: got %v, want %v", gotFalse, wantFalse)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		outTrue := make(chan int)
		outFalse := make(chan int)
		pred := func(v int) bool { return v > 0 }

		go func() {
			Partition(t.Context(), outTrue, outFalse, in, pred)
			close(outTrue)
			close(outFalse)
		}()

		close(in)
		for v := range outTrue {
			t.Errorf("outTrue: should not emit, got %v", v)
		}
		for v := range outFalse {
			t.Errorf("outFalse: should not emit, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		outTrue := make(chan int, 4)
		outFalse := make(chan int, 4)
		pred := func(v int) bool { return v%2 == 0 }

		go func() {
			Partition(ctx, outTrue, outFalse, in, pred)
			close(outTrue)
			close(outFalse)
		}()

		in <- 2
		in <- 3
		<-outTrue
		<-outFalse
		cancel()

		in <- 4
		in <- 5
		close(in)

		for v := range outTrue {
			t.Errorf("outTrue: should not emit after cancel, got %v", v)
		}
		for v := range outFalse {
			t.Errorf("outFalse: should not emit after cancel, got %v", v)
		}
	})
}

func TestReduce(t *testing.T) {
	t.Run("sum ints", func(t *testing.T) {
		in := make(chan int, 5)
		fn := func(acc, v int) int { return acc + v }

		for i := 1; i <= 5; i++ {
			in <- 10*i + i
		}
		close(in)

		got := Reduce(t.Context(), in, 0, fn)
		want := 11 + 22 + 33 + 44 + 55
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("concat strings", func(t *testing.T) {
		in := make(chan string, 3)
		fn := func(acc, v string) string { return acc + v }

		in <- "a"
		in <- "b"
		in <- "c"
		close(in)

		got := Reduce(t.Context(), in, "", fn)
		want := "abc"
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		fn := func(acc, v int) int { return acc + v }

		close(in)

		got := Reduce(t.Context(), in, 42, fn)
		want := 42
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 1)
		fn := func(acc, v int) int { return acc + v }
		done := make(chan struct{}, 2)

		var total atomic.Int32
		go func() {
			got := Reduce(ctx, in, 0, fn)
			total.Store(int32(got))
			done <- struct{}{}
		}()

		in <- 11
		in <- 22
		cancel()

		go func() {
			in <- 33
			select {
			case in <- 44:
			default:
			}
			close(in)
			done <- struct{}{}
		}()

		<-done
		<-done
		got := int(total.Load())
		want := 33
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestSplit(t *testing.T) {
	t.Run("zero chans", func(t *testing.T) {
		in := make(chan int, 3)
		outs := []chan<- int{}
		done := make(chan struct{})

		go func() {
			defer close(done)
			Split(t.Context(), outs, in)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		<-done
		if len(in) != 0 {
			t.Error("should drain the input")
		}
	})
	t.Run("one chan", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)
		outs := []chan<- int{out}

		for i := 1; i <= 5; i++ {
			in <- 10*i + i
		}
		close(in)

		go func() {
			Split(t.Context(), outs, in)
			close(out)
		}()

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33, 44, 55}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("out: got %v, want %v", got, want)
		}
	})
	t.Run("two chans", func(t *testing.T) {
		in := make(chan int, 6)
		out1 := make(chan int, 3)
		out2 := make(chan int, 3)
		outs := []chan<- int{out1, out2}

		for i := 1; i <= 6; i++ {
			in <- 10*i + i
		}
		close(in)

		go func() {
			Split(t.Context(), outs, in)
			close(out1)
			close(out2)
		}()

		var got1 []int
		for v := range out1 {
			got1 = append(got1, v)
		}
		want1 := []int{11, 33, 55}
		if !reflect.DeepEqual(got1, want1) {
			t.Errorf("out 1: got %v, want %v", got1, want1)
		}

		var got2 []int
		for v := range out2 {
			got2 = append(got2, v)
		}
		want2 := []int{22, 44, 66}
		if !reflect.DeepEqual(got2, want2) {
			t.Errorf("out 2: got %v, want %v", got2, want2)
		}
	})
	t.Run("five chans", func(t *testing.T) {
		in := make(chan int, 5)
		out1 := make(chan int, 1)
		out2 := make(chan int, 1)
		out3 := make(chan int, 1)
		out4 := make(chan int, 1)
		out5 := make(chan int, 1)

		for i := 1; i <= 5; i++ {
			in <- 10*i + i
		}
		close(in)

		go func() {
			outs := []chan<- int{out1, out2, out3, out4, out5}
			Split(t.Context(), outs, in)
			for _, out := range outs {
				close(out)
			}
		}()

		outs := []<-chan int{out1, out2, out3, out4, out5}
		for i, out := range outs {
			var got []int
			for v := range out {
				got = append(got, v)
			}
			want := []int{10*(i+1) + (i + 1)}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("out %d: got %v, want %v", i, got, want)
			}
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		outs := []chan<- int{out}

		go func() {
			Split(t.Context(), outs, in)
			close(out)
		}()

		close(in)
		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out1 := make(chan int, 2)
		out2 := make(chan int, 2)
		outs := []chan<- int{out1, out2}

		go func() {
			Split(ctx, outs, in)
			close(out1)
			close(out2)
		}()

		in <- 11
		in <- 22
		<-out1
		<-out2
		cancel()

		in <- 33
		in <- 44
		close(in)

		for v := range out1 {
			t.Errorf("should not emit after cancel, got %v", v)
		}
		for v := range out2 {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}

func TestTake(t *testing.T) {
	t.Run("n equals zero", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Take(t.Context(), out, in, 0)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		if len(got) != 0 {
			t.Errorf("should not emit anything when n=0, got %v", got)
		}
	})
	t.Run("n equals one", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			Take(t.Context(), out, in, 1)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n less than input", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)

		go func() {
			defer close(out)
			Take(t.Context(), out, in, 3)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		in <- 55
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n greater than input", func(t *testing.T) {
		in := make(chan int, 2)
		out := make(chan int, 2)

		go func() {
			defer close(out)
			Take(t.Context(), out, in, 5)
		}()

		in <- 11
		in <- 22
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			Take(t.Context(), out, in, 3)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out := make(chan int, 4)

		go func() {
			defer close(out)
			Take(ctx, out, in, 4)
		}()

		in <- 11
		in <- 22
		<-out
		<-out
		cancel()

		in <- 33
		in <- 44
		close(in)

		for v := range out {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}

func TestTakeNth(t *testing.T) {
	t.Run("n equals zero", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			TakeNth(t.Context(), out, in, 0)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		for v := range out {
			t.Errorf("should not emit anything when n=0, got %v", v)
		}
	})
	t.Run("n equals one", func(t *testing.T) {
		in := make(chan int, 4)
		out := make(chan int, 4)

		go func() {
			defer close(out)
			TakeNth(t.Context(), out, in, 1)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33, 44}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n equals two", func(t *testing.T) {
		in := make(chan int, 6)
		out := make(chan int, 3)

		go func() {
			defer close(out)
			TakeNth(t.Context(), out, in, 2)
		}()

		in <- 1
		in <- 2
		in <- 3
		in <- 4
		in <- 5
		in <- 6
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{1, 3, 5}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("n greater than input", func(t *testing.T) {
		in := make(chan int, 2)
		out := make(chan int, 2)

		go func() {
			defer close(out)
			TakeNth(t.Context(), out, in, 5)
		}()

		in <- 11
		in <- 22
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)

		go func() {
			defer close(out)
			TakeNth(t.Context(), out, in, 2)
		}()

		close(in)

		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 5)
		out := make(chan int, 5)

		go func() {
			defer close(out)
			TakeNth(ctx, out, in, 2)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		<-out // 11
		<-out // 33
		cancel()

		in <- 55
		close(in)

		for v := range out {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}

func TestTakeWhile(t *testing.T) {
	t.Run("take some", func(t *testing.T) {
		in := make(chan int, 5)
		out := make(chan int, 5)
		keep := func(v int) bool { return v < 33 }

		go func() {
			defer close(out)
			TakeWhile(t.Context(), out, in, keep)
		}()

		in <- 11
		in <- 22
		in <- 33
		in <- 44
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("take none", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		keep := func(v int) bool { return false }

		go func() {
			defer close(out)
			TakeWhile(t.Context(), out, in, keep)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		for v := range out {
			t.Errorf("should not emit, got %v", v)
		}
	})
	t.Run("take all", func(t *testing.T) {
		in := make(chan int, 3)
		out := make(chan int, 3)
		keep := func(v int) bool { return true }

		go func() {
			defer close(out)
			TakeWhile(t.Context(), out, in, keep)
		}()

		in <- 11
		in <- 22
		in <- 33
		close(in)

		var got []int
		for v := range out {
			got = append(got, v)
		}
		want := []int{11, 22, 33}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty input", func(t *testing.T) {
		in := make(chan int)
		out := make(chan int)
		keep := func(v int) bool { return true }

		go func() {
			defer close(out)
			TakeWhile(t.Context(), out, in, keep)
		}()

		close(in)
		for v := range out {
			t.Errorf("should not emit anything, got %v", v)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan int, 4)
		out := make(chan int, 4)
		keep := func(v int) bool { return true }

		go func() {
			defer close(out)
			TakeWhile(ctx, out, in, keep)
		}()

		in <- 11
		in <- 22
		<-out
		<-out
		cancel()

		in <- 33
		in <- 44
		close(in)

		for v := range out {
			t.Errorf("should not emit after cancel, got %v", v)
		}
	})
}
