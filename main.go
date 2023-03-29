package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var videoExts = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".flv":  true,
	".wmv":  true,
	".webm": true,
}

var subtitleExts = map[string]bool{
	".srt": true,
	".ass": true,
	".ssa": true,
	".sub": true,
	".idx": true,
}

func main() {
	folder := flag.String("folder", "./", "待转换的文件夹路径")
	crf := flag.Int("crf", 28, "视频编码质量 (H.265 CRF)")
	threads := flag.Int("threads", 0, "编码线程数,0是最佳")
	codec := flag.String("codec", "hevc", "视频编码器。可选项: hevc, hevc_qsv, hevc_amf, hevc_nvenc, h264, vp9")
	subtitles := flag.Bool("subtitles", false, "是否嵌入字幕")
	preset := flag.String("preset", "medium", "视频编码器预设。可选项: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [参数]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "参数列表:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	encoderAvailable := false
	encodersCmd := exec.Command("ffmpeg", "-encoders")
	out, err := encodersCmd.Output()

	if err != nil {
		fmt.Println("检查编码器列表时出错: ", err)
		os.Exit(1)
	}

	if strings.Contains(string(out), *codec) {
		encoderAvailable = true
	}

	if !encoderAvailable {
		fmt.Printf("指定的编码器 %s 不可用，请选择其他编码器。", *codec)
		os.Exit(1)
	}

	err = filepath.Walk(*folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		outputPath := strings.TrimSuffix(path, ext) + "_" + *codec + ".mp4"

		var subtitlePath string
		if *subtitles {
			for subtitleExt := range subtitleExts {
				subtitlePath = strings.TrimSuffix(path, ext) + subtitleExt
				if _, err := os.Stat(subtitlePath); err == nil {
					break
				} else {
					subtitlePath = ""
				}
			}
		}

		cmdArgs := []string{"-i", path, "-c:v", *codec, "-preset", *preset, "-threads", fmt.Sprintf("%d", *threads), "-crf", fmt.Sprintf("%d", *crf), "-c:a", "copy", "-y"}
		if subtitlePath != "" {
			cmdArgs = append(cmdArgs, "-vf", "subtitles="+subtitlePath)
		}
		cmdArgs = append(cmdArgs, outputPath)
		cmd := exec.Command("ffmpeg", cmdArgs...)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		if err := cmd.Start(); err != nil {
			return err
		}

		// 启动 goroutine 循环读取标准错误流
		go func() {
			defer stderr.Close()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				// 解析进度信息，例如：frame=  100 fps= 10 q=10.0 ...
				if strings.Contains(line, "time=") && strings.Contains(line, "duration=") {
					// 解析当前时间和总时间
					reTime := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d{2})`)
					reDuration := regexp.MustCompile(`duration=(\d+\.\d+)`)
					timeMatches := reTime.FindStringSubmatch(line)
					durationMatches := reDuration.FindStringSubmatch(line)
					if len(timeMatches) == 4 && len(durationMatches) == 2 {
						hours, _ := strconv.Atoi(timeMatches[1])
						minutes, _ := strconv.Atoi(timeMatches[2])
						seconds, _ := strconv.ParseFloat(timeMatches[3], 64)
						current := float64(hours)*3600 + float64(minutes)*60 + seconds
						duration, _ := strconv.ParseFloat(durationMatches[1], 64)

						// 计算当前进度并输出
						progress := current / duration * 100
						fmt.Printf("\r转码进度：%.2f%%", progress)
					}
				}
				fmt.Println(line)
			}
		}()

		if err := cmd.Wait(); err != nil {
			return err
		}

		fmt.Printf("转换完成: %s 到 %s\n", path, outputPath)
		return nil
	})

	if err != nil {
		fmt.Println("转换错误: ", err)
	}
}
