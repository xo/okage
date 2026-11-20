// Package textbench measures the speed of the text code of okage: the
// packages of typesetting, the copy of go-text (D52). It has no code of its
// own. The benchmarks are in the test files, and they use only the public
// API of typesetting, so that a change inside the copy does not break them.
//
// # The workloads
//
// Each benchmark has a sub-benchmark for each workload:
//
//   - pg24264 is gutenberg/txt/pg24264.txt, a novel in Chinese, as ivy lays
//     out a text file: the lines of a paragraph join, in a monospace font of
//     13px on a line of 672px. MPLUS1p gives the ideographs, and it is the
//     fallback face of the characters that it does not have.
//   - pg1342 is gutenberg/html/pg1342-images.html, a novel in English: the
//     text of each element p, in Noto Sans of 16px on a line of 624px.
//   - english, arabic, mixed, devanagari, emoji and variable are small texts
//     in this package: English prose, Arabic from right to left, English with
//     Arabic words and numbers, Hindi with its combining marks, emoji
//     sequences, and English in the variable font Roboto Flex.
//
// The two books are not in okage. The benchmarks read them from the folder
// that the environment variable OKAGE_GUTENBERG names. That folder holds
// txt/pg24264.txt and html/pg1342-images.html. If the variable is not set,
// the benchmarks read ../../../ivy/test/review/cache/files/gutenberg, which
// is the cache of ivy in a checkout of ivy next to okage. If a book is not
// there, its sub-benchmarks are skipped.
//
// # The stages
//
// These benchmarks measure one stage each, on all the paragraphs of a
// workload:
//
//   - BenchmarkSegmenter: the break attributes of segmenter, and the line
//     breaks that its iterator gives.
//   - BenchmarkBidi: the bidi algorithm on each paragraph.
//   - BenchmarkSplit: shaping.Segmenter.Split, which splits the text by
//     direction, script and face. The time of the split by script is about
//     the time of BenchmarkSplit less the times of BenchmarkBidi and
//     BenchmarkSplitByFace.
//   - BenchmarkSplitByFace: shaping.SplitByFace alone.
//   - BenchmarkShape and BenchmarkShapeInto: the shaper of harfbuzz on the
//     runs of Split.
//   - BenchmarkWrap: shaping.LineWrapper breaks each shaped paragraph into
//     lines of a fixed width.
//
// BenchmarkEndToEnd models the path of ivy, after D70. For each line, it
// shapes a short text from the start of the rest of the paragraph, asks the
// segmenter for its line breaks, and finds the first line. Then it takes the
// glyphs of the line from the short text when the cut is safe to break, and
// splits the line again to make sure of its runs, as ivy does. Else it
// shapes the line again. BenchmarkEndToEndWrap is the path of go-text
// itself: Split, Shape and LineWrapper, once for each paragraph.
//
// Each benchmark reports its allocations and ns/char, the time for each
// character of the workload. BenchmarkEndToEnd also reports shaped/char,
// the number of characters that it shapes for each character of the
// workload.
//
// # How to measure
//
// Run the benchmarks from the folder test of okage. Run each one 6 times,
// so that benchstat can give a confidence interval:
//
//	export OKAGE_GUTENBERG=/path/to/gutenberg
//	go test -run '^$' -bench . -count 6 ./textbench/ | tee old.txt
//
// Make the change, then run the same command into new.txt. Compare the two
// runs with benchstat:
//
//	go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
//
// To find where the time goes, profile one benchmark:
//
//	go test -run '^$' -bench 'EndToEnd$/^pg24264$' -benchtime 10x \
//	  -cpuprofile cpu.out -memprofile mem.out ./textbench/
//	go tool pprof -top -nodecount 40 cpu.out
//	go tool pprof -top -cum -nodecount 40 cpu.out
//	go tool pprof -sample_index alloc_space -top -nodecount 40 mem.out
//
// A change to the copy of go-text needs a decision first (D52). A benchmark
// that gets faster is not enough: the output of each test and the hashes of
// ivy must stay the same.
package textbench
