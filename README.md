# chans: Building blocks for idiomatic Go pipelines

The `chans` package provides generic channel operations to help you build concurrent pipelines in Go. It aims to be flexible, unopinionated, and composable, without over-abstracting or taking control away from the developer.

```go
ctx := context.Background()
words := make(chan string, 3)

words <- "go"
words <- "is"
words <- "awesome"
close(words)

fn := func(acc int, word string) int { return acc + len(word) }
count := chans.Reduce(ctx, words, 0, fn)

fmt.Println("char count = ", count)
// char count =  11
```

## Features

The golden trio:

-   `Filter`: Sends values from the input channel to the output if a predicate returns true.
-   `Map`: Reads values from the input channel, applies a function, and sends the result to the output.
-   `Reduce`: Combines all values from the input channel into one using a function and returns the result.

Filtering and sampling:

-   `FilterOut`: Ignores values from the input channel if a predicate returns true, otherwise sends them to the output.
-   `Drop`: Skips the first N values from the input channel and sends the rest to the output.
-   `DropWhile`: Skips values from the input channel as long as a predicate returns true, then sends the rest to the output.
-   `Take`: Sends up to N values from the input channel to the output.
-   `TakeNth`: Sends every Nth value from the input channel to the output.
-   `TakeWhile`: Sends values from the input channel to the output while a predicate returns true.
-   `First`: Returns the first value from the input channel that matches a predicate.

Batching and windowing:

-   `Chunk`: Groups values from the input channel into fixed-size slices and sends them to the output.
-   `ChunkBy`: Groups consecutive values from the input channel into slices whenever the key function's result changes.
-   `Flatten`: Reads slices from the input channel and sends their elements to the output in order.

De-duplication:

-   `Compact`: Sends values from the input channel to the output, skipping consecutive duplicates.
-   `CompactBy`: Sends values from the input channel to the output, skipping consecutive duplicates as determined by a custom equality function.
-   `Distinct`: Sends values from the input channel to the output, skipping all duplicates.
-   `DistinctBy`: Sends values from the input channel to the output, skipping duplicates as determined by a key function.

Routing:

-   `Broadcast`: Sends every value from the input channel to all output channels.
-   `Split`: Sends values from the input channel to output channels in round-robin fashion.
-   `Partition`: Sends values from the input channel to one of two outputs based on a predicate.
-   `Merge`: Concurrently sends values from multiple input channels to the output, with no guaranteed order.
-   `Concat`: Sends values from multiple input channels to the output, processing each input channel in order.
-   `Drain`: Consumes and discards all values from the input channel.

## Motivation

I think third-party concurrency packages are often too opinionated and try to hide too much complexity. As a result, they end up being inflexible and don't fit a lot of use cases.

For example, here's how you use the `Map` function from the [rill](https://github.com/destel/rill) package:

```go
// Concurrency = 3
users := rill.Map(ids, 3, func(id int) (*User, error) {
    return db.GetUser(ctx, id)
})
```

The code looks simple, but it makes `Map` pretty opinionated and not very flexible:

-   The function is non-blocking and spawns a goroutine. There is no way to change this.
-   The function doesn't exit early on error. There is no way to change this.
-   The function creates the output channel. There is no way to control its buffering or lifecycle.
-   The function can't be canceled.
-   The function requires the developer to use a custom `Try[T]` type for both input and output channels.
-   The "N workers" logic is baked in, so you can't use a custom concurrent group implementation.

While this approach works for many developers, I personally don't like it. With `chans`, my goal was to offer a fairly low-level set of composable channel operations and let developers decide how to use them.

For comparison, here's how you use the `chans.Map` function:

```go
err := chans.Map(ctx, users, ids, func(id int) (*User, error) {
    return db.GetUser(ctx, id)
})
```

`chans.Map` only implements the core mapping logic:

-   Reads values from the input channel.
-   Calls the mapping function on each value.
-   Writes results to the output channel.
-   Stops if there's an error or if the context is canceled.
-   Does not start any additional goroutines.

You decide the rest:

-   Want Map to be non-blocking? Run it in a goroutine.
-   Don't want to exit early? Gather the errors instead of returning them.
-   Want to buffer the output channel or keep it opened? You have full control.
-   Need to process input in parallel? Use `errgroup.Group`, or `sync.WaitGroup`, or any other implementation.

The same applies to other channel operations.

## Contributing

Contributions are welcome. For anything other than bug fixes, please open an issue first to discuss what you want to change.

Make sure to add or update tests as needed.

## License

Created by [Anton Zhiyanov](https://antonz.org/). Released under the MIT License.
