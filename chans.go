// Package chans provides generic channel functions useful for building concurrent pipelines.
//
// Concurrency:
//   - All functions are synchronous: they run in the caller's goroutine and block until
//     the input channel is drained or the context is canceled. The only function that
//     creates additional goroutines is Merge.
//   - Callers own the input and output channels. The package functions never try to close them.
//   - All sends/receives are blocking.
//
// Cancellation:
//   - All functions accept a context and stop early if the context is canceled.
//   - Functions that return an error (Map, Filter, FilterOut) return ctx.Err() on cancellation.
//   - Other functions return immediately without an error; check ctx.Err() after return
//     to distinguish normal completion from cancellation.
//
// Unless otherwise noted, all arguments must be non-nil, and output channels
// must not be closed while a function is writing to them.
//
// All functions use O(1) memory unless otherwise noted.
package chans

import "context"

// Broadcast sends every value from in to all out channels, in index order.
// Blocks on each out; a single slow out will stall all.
// If outs is empty, drains in and discards.
func Broadcast[V any](ctx context.Context, outs []chan<- V, in <-chan V) {
	if len(outs) == 0 {
		// Drain and discard.
		forEach(ctx, in, func(V) {})
		return
	}
	forEach(ctx, in, func(item V) {
		for _, out := range outs {
			select {
			case out <- item:
			case <-ctx.Done():
				return
			}
		}
	})
}

// Chunk groups values from in into consecutive slices of size n and sends them to out.
// The last slice may have fewer than n items.
// If n<=0, Chunk returns immediately without sending anything to out.
//
// Uses O(n) memory.
func Chunk[V any](ctx context.Context, out chan<- []V, in <-chan V, n int) {
	if n <= 0 {
		return
	}

	// Write all chunks except the last one.
	chunk := make([]V, 0, n)
	forEach(ctx, in, func(item V) {
		chunk = append(chunk, item)
		if len(chunk) == n {
			select {
			case out <- chunk:
				chunk = make([]V, 0, n)
			case <-ctx.Done():
				return
			}
		}
	})

	// Return if canceled.
	if ctx.Err() != nil {
		return
	}

	// Write the last chunk.
	if len(chunk) > 0 {
		select {
		case out <- chunk:
		case <-ctx.Done():
		}
	}
}

// ChunkBy groups values from in into consecutive slices whenever
// the result of the key function changes, and sends them to out.
func ChunkBy[V any, K comparable](ctx context.Context, out chan<- []V, in <-chan V, key func(V) K) {
	var (
		chunk   []V
		prevKey K
		hasPrev bool
	)

	// Write all chunks except the last one.
	forEach(ctx, in, func(item V) {
		k := key(item)
		if hasPrev && k != prevKey && len(chunk) > 0 {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
			chunk = nil
		}
		chunk = append(chunk, item)
		prevKey = k
		hasPrev = true
	})

	// Return if canceled.
	if ctx.Err() != nil {
		return
	}

	// Write the last chunk.
	if len(chunk) > 0 {
		select {
		case out <- chunk:
		case <-ctx.Done():
		}
	}
}

// Compact sends values from in to out, skipping consecutive duplicates.
func Compact[V comparable](ctx context.Context, out chan<- V, in <-chan V) {
	var prev V
	var hasPrev bool
	forEach(ctx, in, func(item V) {
		if hasPrev && prev == item {
			return
		}
		select {
		case out <- item:
			prev = item
			hasPrev = true
		case <-ctx.Done():
			return
		}
	})
}

// CompactBy sends values from in to out, skipping consecutive duplicates as determined by eq.
func CompactBy[V any](ctx context.Context, out chan<- V, in <-chan V, eq func(V, V) bool) {
	var prev V
	var hasPrev bool
	forEach(ctx, in, func(item V) {
		if hasPrev && eq(prev, item) {
			return
		}
		select {
		case out <- item:
			prev = item
			hasPrev = true
		case <-ctx.Done():
			return
		}
	})
}

// Concat sends values from multiple input channels to out.
// Processes each channel one after the other: first the first input channel, then the second, then the third, and so on.
// Unlike [Merge], Concat does not spawn additional goroutines.
func Concat[V any](ctx context.Context, out chan<- V, ins ...<-chan V) {
	for _, in := range ins {
		forEach(ctx, in, func(item V) {
			select {
			case out <- item:
			case <-ctx.Done():
				return
			}
		})
		if ctx.Err() != nil {
			return
		}
	}
}

// Distinct sends values from in to out, skipping duplicates.
//
// Uses O(u) memory, where u is the number of distinct values seen.
func Distinct[V comparable](ctx context.Context, out chan<- V, in <-chan V) {
	seen := make(map[V]struct{})
	forEach(ctx, in, func(item V) {
		if _, exists := seen[item]; exists {
			return
		}
		select {
		case out <- item:
			seen[item] = struct{}{}
		case <-ctx.Done():
			return
		}
	})
}

// DistinctBy sends values from in to out, skipping duplicates as determined by key.
// The key function extracts a comparable key from each value for O(1) duplicate checks.
//
// Uses O(u) memory, where u is the number of distinct keys seen.
func DistinctBy[V any, K comparable](ctx context.Context, out chan<- V, in <-chan V, key func(V) K) {
	seen := make(map[K]struct{})
	forEach(ctx, in, func(item V) {
		k := key(item)
		if _, exists := seen[k]; exists {
			return
		}
		select {
		case out <- item:
			seen[k] = struct{}{}
		case <-ctx.Done():
			return
		}
	})
}

// Drain consumes and discards all values from in.
func Drain[V any](ctx context.Context, in <-chan V) {
	forEach(ctx, in, func(V) {})
}

// Drop skips the first n values from in and sends the rest to out.
// If n<=0, sends all values from in to out.
func Drop[V any](ctx context.Context, out chan<- V, in <-chan V, n int) {
	count := 0
	forEach(ctx, in, func(item V) {
		if count < n {
			count++
			return
		}
		select {
		case out <- item:
		case <-ctx.Done():
			return
		}
	})
}

// DropWhile skips the first values from in as long as drop returns true, then sends the rest to out.
func DropWhile[V any](ctx context.Context, out chan<- V, in <-chan V, drop func(V) bool) {
	dropping := true
	forEach(ctx, in, func(item V) {
		if dropping && drop(item) {
			return
		}
		dropping = false
		select {
		case out <- item:
		case <-ctx.Done():
			return
		}
	})
}

// Filter sends values from in to out if keep returns true.
// If keep returns an error, Filter stops immediately and returns that error.
// If the context is canceled, Filter stops immediately and returns the context error.
func Filter[V any](ctx context.Context, out chan<- V, in <-chan V, keep func(V) (bool, error)) error {
	return untilErr(ctx, in, func(item V) error {
		keeping, err := keep(item)
		if err != nil {
			return err
		}
		if !keeping {
			return nil
		}
		select {
		case out <- item:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// FilterOut ignores values from in if drop returns true, otherwise sends them to out.
// If drop returns an error, FilterOut stops immediately and returns that error.
// If the context is canceled, FilterOut stops immediately and returns the context error.
func FilterOut[V any](ctx context.Context, out chan<- V, in <-chan V, drop func(V) (bool, error)) error {
	return untilErr(ctx, in, func(item V) error {
		dropping, err := drop(item)
		if err != nil {
			return err
		}
		if dropping {
			return nil
		}
		select {
		case out <- item:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// First returns the first value from in that matches pred.
// If no value matches, returns the zero value and false.
// If pred is nil, returns the first value from in.
// On cancellation, returns the zero value and false.
// Check ctx.Err() after return to distinguish cancellation from "not found".
func First[V any](ctx context.Context, in <-chan V, pred func(V) bool) (V, bool) {
	var first V
	var found bool
	whileTrue(ctx, in, func(item V) bool {
		if pred == nil || pred(item) {
			first = item
			found = true
			return false
		}
		return true
	})
	return first, found
}

// Flatten reads slices from in and sends their elements to out, in order.
func Flatten[V any](ctx context.Context, out chan<- V, in <-chan []V) {
	forEach(ctx, in, func(chunk []V) {
		for _, item := range chunk {
			select {
			case out <- item:
			case <-ctx.Done():
				return
			}
		}
	})
}

// Map reads values from in, applies fn to each value, and sends the result to out.
// If fn returns an error, Map stops immediately and returns that error.
// If the context is canceled, Map stops immediately and returns the context error.
func Map[V, U any](ctx context.Context, out chan<- U, in <-chan V, fn func(V) (U, error)) error {
	return untilErr(ctx, in, func(item V) error {
		val, err := fn(item)
		if err != nil {
			return err
		}
		select {
		case out <- val:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// Merge sends values from multiple input channels to out.
// The merge is done concurrently in multiple goroutines. The order of values in out is not guaranteed.
// Blocks on out; a slow consumer will stall all inputs.
// Merge returns when all input channels are closed or the context is canceled. If there are no input channels, returns immediately.
func Merge[V any](ctx context.Context, out chan<- V, ins ...<-chan V) {
	// Don't spawn goroutines if the context is already canceled.
	if ctx.Err() != nil {
		return
	}

	// Merge input channels.
	done := make(chan struct{}, len(ins))
	for _, in := range ins {
		go func() {
			forEach(ctx, in, func(item V) {
				select {
				case out <- item:
				case <-ctx.Done():
					return
				}
			})
			done <- struct{}{}
		}()
	}

	// Return if canceled.
	if ctx.Err() != nil {
		return
	}

	// Wait for all input channels to be closed or context to be canceled.
	for range ins {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
}

// Partition sends values from in to outTrue if keep returns true, otherwise to outFalse.
func Partition[V any](ctx context.Context, outTrue, outFalse chan<- V, in <-chan V, pred func(V) bool) {
	forEach(ctx, in, func(item V) {
		dst := outFalse
		if pred(item) {
			dst = outTrue
		}
		select {
		case dst <- item:
		case <-ctx.Done():
			return
		}
	})
}

// Split sends values from in to out channels in a round-robin fashion.
// Each value from in is sent to a single out channel.
// Blocks on each out; a single slow out will stall all.
// If outs is empty, drains in and discards.
func Split[V any](ctx context.Context, outs []chan<- V, in <-chan V) {
	if len(outs) == 0 {
		// Drain and discard.
		forEach(ctx, in, func(V) {})
		return
	}
	i := 0
	forEach(ctx, in, func(item V) {
		select {
		case outs[i%len(outs)] <- item:
			i++
		case <-ctx.Done():
			return
		}
	})
}

// Reduce combines all values from in into one using fn and returns the result.
// On cancellation, returns the accumulator computed so far.
// Check ctx.Err() after return to distinguish cancellation from normal completion.
func Reduce[V, U any](ctx context.Context, in <-chan V, init U, fn func(U, V) U) U {
	acc := init
	forEach(ctx, in, func(item V) {
		acc = fn(acc, item)
	})
	return acc
}

// Take sends up to n values from in to out.
// If n<=0, Take returns immediately without sending anything to out.
func Take[V any](ctx context.Context, out chan<- V, in <-chan V, n int) {
	if n <= 0 {
		return
	}
	count := 0
	whileTrue(ctx, in, func(item V) bool {
		select {
		case out <- item:
			count++
			return count < n
		case <-ctx.Done():
			return false
		}
	})
}

// TakeNth sends every nth value from in to out.
// Emits items at indices 0, n, 2n, ...
// If n<=0, TakeNth returns immediately without sending anything to out.
func TakeNth[V any](ctx context.Context, out chan<- V, in <-chan V, n int) {
	if n <= 0 {
		return
	}
	count := 0
	forEach(ctx, in, func(item V) {
		if count%n == 0 {
			select {
			case out <- item:
			case <-ctx.Done():
				return
			}
		}
		count++
	})
}

// TakeWhile sends values from in to out while keep returns true.
func TakeWhile[V any](ctx context.Context, out chan<- V, in <-chan V, keep func(V) bool) {
	whileTrue(ctx, in, func(item V) bool {
		if !keep(item) {
			return false
		}
		select {
		case out <- item:
			return true
		case <-ctx.Done():
			return false
		}
	})
}

// forEach iterates over the input channel, calling fn for each item
// until the context is canceled or the channel is closed.
func forEach[V any](ctx context.Context, in <-chan V, fn func(V)) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-in:
			if !ok {
				return
			}
			fn(item)
		}
	}
}

// whileTrue iterates over the input channel, calling fn for each item
// until fn returns false, or the context is canceled, or the channel is closed.
func whileTrue[V any](ctx context.Context, in <-chan V, fn func(V) bool) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-in:
			if !ok {
				return
			}
			if !fn(item) {
				return
			}
		}
	}
}

// untilErr iterates over the input channel, calling fn for each item
// until fn returns an error, or the context is canceled, or the channel is closed.
func untilErr[V any](ctx context.Context, in <-chan V, fn func(V) error) error {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-in:
			if !ok {
				return nil
			}
			err := fn(item)
			if err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
