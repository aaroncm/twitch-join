package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/cheggaaa/pb"
)

var outfn string
var flvs []string
var tempdir string
var listfh *os.File
var err error

func init() {
	// cmd line usage / args
	flag.Usage = func() {
		fmt.Println("Usage: twitch-join [-o output.flv] input1.flv input2.flv ...")
		fmt.Println("   If output filename is not specified, it will be")
		fmt.Println("   inferred from the input filenames.")
	}
	flag.StringVar(&outfn, "o", "", "output filename")
	flag.Parse()
	flvs = flag.Args()
	if len(flvs) == 0 {
		log.Fatal("Please specify some input flvs (-h for usage)")
	}

	if outfn == "" {
		outfn = getCommonFilename(flvs)
	}
	fmt.Println("output filename:", outfn)

	// create temporary work area
	tempdir, err = ioutil.TempDir("", "twitch-join")
	if err != nil {
		log.Fatal("error creating temp dir", err)
	}
	fmt.Println("created temp dir:", tempdir)

	if listfh, err = ioutil.TempFile(tempdir, "list"); err != nil {
		log.Panic("error creating list file", err)
	}
}

func main() {
	// panic handler, deferred first so it fires after the cleanup defer
	defer func() {
		if r := recover(); r != nil {
			log.Println("panicked, cleaning up...")
			os.Exit(1)
		}
	}()

	removeTempdir := func() {
		err = os.RemoveAll(tempdir)
		if err != nil {
			log.Println("couldn't remove tempdir:", tempdir)
		}
	}

	// clean up tempdir in event of ctrl-c
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		<-sigchan
		removeTempdir()
		panic("interrupted")
	}()

	// also remove tempdir if things exit normally
	defer removeTempdir()

	tempoutfn := path.Join(tempdir, outfn)

	// size is needed for the progress bar, so calculate it while we go
	totalSize := cleanupFLVs(flvs, tempdir, listfh)

	if err = joinFLVs(listfh.Name(), tempoutfn, totalSize); err != nil {
		log.Panic("error joining files: ", err)
	}

	fmt.Println("moving to", outfn)
	if err = os.Rename(tempoutfn, outfn); err != nil {
		log.Panic("error renaming file: ", err)
	}
}

func getCommonFilename(names []string) string {
	sort.Strings(names)
	s1 := names[0]
	s2 := names[len(names)-1]
	for i := range s1 {
		if s1[i] != s2[i] {
			return s1[:i] + "-joined.flv"
		}
	}
	return "joined.flv"
}

func cleanupFLVs(flvs []string, tempdir string, listfh *os.File) (totalSize int) {
	for _, flv := range flvs {
		newflv := path.Join(tempdir, flv)
		newflv = strings.Replace(newflv, "'", "", -1)

		fmt.Println("fixing metadata for", flv)
		cmd := exec.Command("/usr/local/bin/yamdi", "-i", flv, "-o", newflv)
		_, err = cmd.CombinedOutput()
		if err != nil {
			log.Panic("error processing flv: ", err)
		}

		listline := "file '" + newflv + "'\n"
		_, err = listfh.Write([]byte(listline))
		if err != nil {
			log.Fatal("error writing temp file:", err)
		}

		stats, _ := os.Stat(newflv)
		totalSize += int(stats.Size() / 1024)
	}
	err = listfh.Close()
	return
}

func joinFLVs(listfn string, tempoutfn string, totalSize int) error {
	fmt.Println("joining...")
	cmd := exec.Command(
		"ffmpeg",
		"-f", "concat",
		"-i", listfn,
		"-c", "copy",
		tempoutfn)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	buf := bufio.NewReader(stderr)
	if err = cmd.Start(); err != nil {
		return err
	}

	bar := pb.StartNew(totalSize)

	go func() {
		for {
			line, _ := buf.ReadString('\r')
			if strings.HasPrefix(line, "frame=") {
				fields := strings.Fields(line)
				sizeStr := strings.TrimRight(fields[4], "kB")
				size, _ := strconv.ParseInt(sizeStr, 10, 64)
				bar.Set(int(size))
			}
		}
	}()

	err = cmd.Wait()
	bar.Set(totalSize)
	bar.Finish()

	return err
}
