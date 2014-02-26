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

func main() {
    // panic handler, deferred first so it fires after the cleanup defer
    defer func() {
        if r := recover(); r != nil {
            log.Println("panicked, cleaning up...")
            os.Exit(1)
        }
    }()

    flag.Parse()
    flvs := flag.Args()
    if len(flvs) == 0 {
        log.Fatal("hey I need filenames")
    }

    common := getCommonFilename(flvs)
    var outfn string
    if len(common) == 0 {
        outfn = "JOINED.flv"
    } else {
        outfn = common + "-JOINED.flv"
    }
    fmt.Println("output filename: ", outfn)

    tempdir, err := ioutil.TempDir("", "twitch-join")
    if err != nil {
        log.Fatal("error creating temp dir", err)
    }
    fmt.Println("created temp dir:", tempdir)

    defer os.RemoveAll(tempdir)

    // clean up tempdir in event of ctrl-c
    go func() {
        sigchan := make(chan os.Signal, 1)
        signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
        <-sigchan
        os.RemoveAll(tempdir)
        panic("interrupted")
    }()

    tempoutfn := path.Join(tempdir, outfn)

    listfh, err := ioutil.TempFile(tempdir, "list")
    if err != nil {
        log.Panic("error creating list file", err)
    }

    var totalSize int

    for _, flv := range flvs {
        newflv := path.Join(tempdir, flv)
        newflv = strings.Replace(newflv, "'", "", -1)

        fmt.Println("fixing metadata for", flv)
        cmd := exec.Command("/usr/local/bin/yamdi", "-i", flv, "-o", newflv)
        _, err := cmd.CombinedOutput()
        if err != nil {
            log.Panic("error processing flv: ", err)
        }

        listline := "file '" + newflv + "'\n"
        listfh.Write([]byte(listline))

        stats, _ := os.Stat(newflv)
        totalSize += int(stats.Size() / 1024)
    }
    listfh.Close()

    fmt.Println("joining...")
    cmd := exec.Command(
        "ffmpeg",
        "-f", "concat",
        "-i", listfh.Name(),
        "-c", "copy",
        tempoutfn)

    stderr, err := cmd.StderrPipe()
    buf := bufio.NewReader(stderr)
    cmd.Start()

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
    if err != nil {
        log.Panic("error joining files: ", err)
    }
    fmt.Println("moving to ", outfn)
    os.Rename(tempoutfn, outfn)
}

func getCommonFilename(names []string) string {
    sort.Strings(names)
    s1 := names[0]
    s2 := names[len(names)-1]
    for i := range s1 {
        if s1[i] != s2[i] {
            return s1[:i]
        }
    }
    return ""
}
