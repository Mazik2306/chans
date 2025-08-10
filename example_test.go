package chans_test

import (
	"context"
	"fmt"

	"github.com/nalgeon/chans"
)

func ExampleBroadcast() {
	ctx := context.Background()
	in := make(chan int, 3)
	out1 := make(chan int, 3)
	out2 := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	close(in)

	outs := []chan<- int{out1, out2}
	chans.Broadcast(ctx, outs, in)

	fmt.Println(<-out1, <-out1, <-out1)
	fmt.Println(<-out2, <-out2, <-out2)

	// Output:
	// 11 22 33
	// 11 22 33
}

func ExampleChunk() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan []int, 3)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	in <- 55
	close(in)

	chans.Chunk(ctx, out, in, 2)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// [11 22] [33 44] [55]
}

func ExampleChunkBy() {
	ctx := context.Background()
	in := make(chan string, 4)
	out := make(chan []string, 4)

	in <- "a"
	in <- "bb"
	in <- "cc"
	in <- "ddd"
	close(in)

	key := func(v string) int { return len(v) }
	chans.ChunkBy(ctx, out, in, key)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// [a] [bb cc] [ddd]
}

func ExampleCompact() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 3)

	in <- 11
	in <- 11
	in <- 22
	in <- 22
	in <- 11
	close(in)

	chans.Compact(ctx, out, in)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// 11 22 11
}

func ExampleCompactBy() {
	ctx := context.Background()
	in := make(chan string, 5)
	out := make(chan string, 4)

	in <- "a"
	in <- "bb"
	in <- "cc"
	in <- "ddd"
	in <- "e"
	close(in)

	eq := func(a, b string) bool { return len(a) == len(b) }
	chans.CompactBy(ctx, out, in, eq)
	fmt.Println(<-out, <-out, <-out, <-out)

	// Output:
	// a bb ddd e
}

func ExampleConcat() {
	ctx := context.Background()
	in1 := make(chan int, 2)
	in2 := make(chan int, 2)
	out := make(chan int, 4)

	in1 <- 11
	in1 <- 22
	close(in1)

	in2 <- 33
	in2 <- 44
	close(in2)

	chans.Concat(ctx, out, in1, in2)
	fmt.Println(<-out, <-out, <-out, <-out)

	// Output:
	// 11 22 33 44
}

func ExampleDistinct() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 2)

	in <- 11
	in <- 11
	in <- 22
	in <- 22
	in <- 11
	close(in)

	chans.Distinct(ctx, out, in)
	fmt.Println(<-out, <-out)

	// Output:
	// 11 22
}

func ExampleDistinctBy() {
	ctx := context.Background()
	in := make(chan string, 5)
	out := make(chan string, 3)

	in <- "a"
	in <- "bb"
	in <- "cc"
	in <- "ddd"
	in <- "e"
	close(in)

	key := func(v string) int { return len(v) }
	chans.DistinctBy(ctx, out, in, key)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// a bb ddd
}

func ExampleDrain() {
	ctx := context.Background()
	in := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	close(in)

	chans.Drain(ctx, in)
	fmt.Println(len(in))

	// Output:
	// 0
}

func ExampleDrop() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	in <- 55
	close(in)

	chans.Drop(ctx, out, in, 2)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// 33 44 55
}

func ExampleDropWhile() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	in <- 22
	in <- 11
	close(in)

	drop := func(x int) bool { return x < 33 }
	chans.DropWhile(ctx, out, in, drop)

	fmt.Println(<-out, <-out, <-out)

	// Output:
	// 33 22 11
}

func ExampleFilter() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 2)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	in <- 55
	close(in)

	keep := func(x int) (bool, error) { return x%2 == 0, nil }
	err := chans.Filter(ctx, out, in, keep)
	fmt.Println(<-out, <-out, err)

	// Output:
	// 22 44 <nil>
}

func ExampleFilterOut() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	in <- 55
	close(in)

	drop := func(x int) (bool, error) { return x%2 == 0, nil }
	err := chans.FilterOut(ctx, out, in, drop)
	fmt.Println(<-out, <-out, <-out, err)

	// Output:
	// 11 33 55 <nil>
}

func ExampleFirst() {
	ctx := context.Background()
	in := make(chan int, 4)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	close(in)

	pred := func(x int) bool { return x > 22 }
	v, ok := chans.First(ctx, in, pred)
	fmt.Println(v, ok)

	// Output:
	// 33 true
}

func ExampleFlatten() {
	ctx := context.Background()
	in := make(chan []int, 2)
	out := make(chan int, 4)

	in <- []int{11, 22}
	in <- []int{33, 44}
	close(in)

	chans.Flatten(ctx, out, in)
	fmt.Println(<-out, <-out, <-out, <-out)

	// Output:
	// 11 22 33 44
}

func ExampleMap() {
	ctx := context.Background()
	in := make(chan string, 3)
	out := make(chan int, 3)

	in <- "a"
	in <- "bb"
	in <- "ccc"
	close(in)

	fn := func(s string) (int, error) { return len(s), nil }
	err := chans.Map(ctx, out, in, fn)
	fmt.Println(<-out, <-out, <-out, err)

	// Output:
	// 1 2 3 <nil>
}

func ExampleMerge() {
	ctx := context.Background()
	in1 := make(chan int, 2)
	in2 := make(chan int, 2)
	out := make(chan int, 4)

	in1 <- 11
	in1 <- 22
	close(in1)

	in2 <- 33
	in2 <- 44
	close(in2)

	chans.Merge(ctx, out, in1, in2)
	fmt.Println(len(out)) // can be any order

	// Output:
	// 4
}

func ExamplePartition() {
	ctx := context.Background()
	in := make(chan int, 4)
	outTrue := make(chan int, 2)
	outFalse := make(chan int, 2)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	close(in)

	pred := func(v int) bool { return v%2 == 0 }
	chans.Partition(ctx, outTrue, outFalse, in, pred)

	fmt.Println(<-outTrue, <-outTrue)
	fmt.Println(<-outFalse, <-outFalse)

	// Output:
	// 22 44
	// 11 33
}

func ExampleReduce() {
	ctx := context.Background()
	in := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	close(in)

	fn := func(acc, x int) int { return acc + x }
	total := chans.Reduce(ctx, in, 0, fn)
	fmt.Println(total)

	// Output:
	// 66
}

func ExampleSplit() {
	ctx := context.Background()
	in := make(chan int, 4)
	out1 := make(chan int, 2)
	out2 := make(chan int, 2)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	close(in)

	outs := []chan<- int{out1, out2}
	chans.Split(ctx, outs, in)

	fmt.Println(<-out1, <-out1)
	fmt.Println(<-out2, <-out2)
	// Output:
	// 11 33
	// 22 44
}

func ExampleTake() {
	ctx := context.Background()
	in := make(chan int, 4)
	out := make(chan int, 2)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	close(in)

	chans.Take(ctx, out, in, 2)
	fmt.Println(<-out, <-out)

	// Output:
	// 11 22
}

func ExampleTakeNth() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 3)

	in <- 11
	in <- 22
	in <- 33
	in <- 44
	in <- 55
	close(in)

	chans.TakeNth(ctx, out, in, 2)
	fmt.Println(<-out, <-out, <-out)

	// Output:
	// 11 33 55
}

func ExampleTakeWhile() {
	ctx := context.Background()
	in := make(chan int, 5)
	out := make(chan int, 2)

	in <- 11
	in <- 22
	in <- 33
	in <- 22
	in <- 11
	close(in)

	keep := func(x int) bool { return x < 33 }
	chans.TakeWhile(ctx, out, in, keep)
	fmt.Println(<-out, <-out)

	// Output:
	// 11 22
}
