# twitch-join
Join twitch.tv's chunked FLVs into single files

I often use [youtube-dl](https://github.com/rg3/youtube-dl) to download archived videos from [twitch.tv](http://twitch.tv). However, their system stores each stream as a series of half-hour `.flv` chunks. It's annoying to have to queue those individually, and while `ffmpeg` can join them into one file, the process is tedious to perform by hand and it doesn't work without preprocessing the files to fix some FLV metadata. This automates the process.

## Requirements
* [go](http://golang.org)
* [yamdi](https://github.com/ioppermann/yamdi)
* [ffmpeg](http://www.ffmpeg.org/)


## Usage
`go get github.com/aaroncm/twitch-join`

`twitch-join [-o output.flv] input1.flv input2.flv ...`

If an output filename is not specified, one will be inferred from the common characters in the names of the input files. Lacking those, it will simply be `joined.flv`.