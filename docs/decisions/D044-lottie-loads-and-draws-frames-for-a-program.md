# D44. `lottie` loads files and draws frames for a program

Status: Decided, amends D41.

D41 gives `lottie` the API of velato and one helper that draws a frame onto a
pixmap. A program that shows Lottie files needs more than that. It must find a
Lottie document among other JSON files, read a dotLottie archive, count and
time the frames, and draw a frame at the size that it shows. The image viewer
iv had this code in its own decoder. The maintainers want the parts that any
program can use in okage, and only the parts that are particular to iv in iv.

## The decision

1. `lottie` adds these APIs, which velato does not have:

   | API | What it does |
   | --- | --- |
   | `Detect(head []byte) bool` | Reports whether the start of a document is a Lottie document. Each of the top-level keys `fr`, `ip`, `op`, `w` and `h` must hold a number. `head` can be cut short at any byte |
   | `IsDotLottie(head []byte) bool` | Reports whether the start of the data is a zip archive, and so a dotLottie archive |
   | `OpenDotLottie(r io.ReaderAt, size int64) (*DotLottie, error)` | Reads the directory of a dotLottie archive. `Animations` holds the sorted paths of its documents: the `.json` files of `animations/`, or, if there are none, the other `.json` files except `manifest.json` |
   | `DotLottie.FS`, `DotLottie.Data`, `DotLottie.Parse` | Give the files of the archive, the bytes of one file, and the composition of one document |
   | `Load(data []byte) (*Composition, error)` | Parses a Lottie document, or the first document of a dotLottie archive |
   | `Composition.FrameCount`, `FrameDuration`, `Duration` | Count the frames, and give the time of one frame and of all of them, from `Frames` and `FrameRate` |
   | `Composition.Image(n, w, h int) (*image.RGBA, error)` | Draws frame `n`, counted from `Frames[0]`, into a new image of `w` by `h` pixels. The composition is scaled to fill the image |

2. The root package adds the sentinel errors `ErrLottieEmpty`, for a
   composition with no size or no frames and for a size that is not
   positive, and `ErrLottieDotLottie`, for an archive that okage cannot read
   or that has no document (D21).
3. The files `detect.go`, `dotlottie.go` and `frame.go` hold these APIs. They
   port no file of velato (D41).
4. These parts stay in iv: the registration of the decoder and its MIME
   types, the size of the start of a file that iv reads to detect a format,
   the choice of the middle frame as the still image, the choice of a page,
   and the playback of an animation.
5. `OpenDotLottie` does not read `manifest.json`. A later decision can add
   its fields, such as the ids, the loop, the speed and the themes, when a
   program needs them.

## When upstream changes

These APIs are additions, not differences in the behavior of velato. If velato
adds an API that does the same work, such as a reader of dotLottie archives,
follow these steps:

1. Port the API of upstream as `docs/UPSTREAM.md` says.
2. If the API of upstream gives the same result, make the API of this
   decision call it, or remove the API of this decision with a note in the
   release notes.
3. If the result is not the same, a maintainer decides which one okage keeps.
   Record the result here or in a new decision.
4. Update the list of added APIs in `README.md`.

## Why

A Lottie file has the extension `.json` and the MIME type of JSON, so a
program that opens files of any type must read the document to know that it
is a Lottie file. Each program that shows Lottie files needs the detection,
the dotLottie archive and the frame helpers, and none of them depends on the
program. The playback and the choice of a frame are choices of a viewer, so
they stay in iv.
