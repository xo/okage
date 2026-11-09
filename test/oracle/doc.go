// Package oracle compares okage with the Rust oracle in the folder rust (D22).
//
// The tests make random scenes of the renderer and random questions of the
// package path. okage and tiny-skia answer each one, and the tests compare
// every byte. The tests skip when cargo cannot build the oracle. The package
// has no code of its own.
//
// The flags of the tests:
//
//	-oracle.n       the number of random scenes and paths (default 2000)
//	-oracle.seed    the seed of the random scenes (default 1)
//	-oracle.replay  a pattern of saved scenes to replay, such as
//	                ../testdata/oracle/*.json
//
// A scene without a cubic curve that differs is saved in testdata/oracle/ in
// the root of the test module, so that the flag -oracle.replay can run it
// again. D26 allows 1 percent of the scenes with a cubic curve to differ, so
// such a scene is saved only if more differ.
package oracle
