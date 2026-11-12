// Package lottie compares the package lottie of okage with ThorVG, through
// the package github.com/xo/lottie, which runs ThorVG on wazero (D41, D42,
// D43).
//
// The test renders five frames of each file of the Noto animated emoji with
// both, and counts the pixels that differ, with the colors premultiplied. A
// pixel differs when one of its channels lies more than 16 below the least or
// above the most of that channel in the pixels of the other image at it and
// next to it, in either direction. So the antialiasing of an edge does not
// count, and a wrong color or a missing shape does. A frame passes when at
// most 1 percent of its pixels differ.
//
// The corpus is in testdata/emoji, which git ignores. fetch-corpus.sh
// downloads it, or the variable OKAGE_LOTTIE_CORPUS names another folder.
// The test skips when the folder holds no file. The package has no code of
// its own.
package lottie
