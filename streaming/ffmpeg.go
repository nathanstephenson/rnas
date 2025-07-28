package streaming

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

type FFMpegOutput struct {
	IsSource     bool
	Width        int
	Height       int
	AudioBitrate int
}

var p1080 = FFMpegOutput{Width: 1920, Height: 1080, AudioBitrate: 256, IsSource: false}
var p720 = FFMpegOutput{Width: 1280, Height: 720, AudioBitrate: 192, IsSource: false}
var p360 = FFMpegOutput{Width: 640, Height: 360, AudioBitrate: 128, IsSource: false}

func getFFMpegArgs(width int, height int) (filterComplex, videoMap, audioMap, buffMap []string) {
	source := FFMpegOutput{Width: width, Height: height, AudioBitrate: 320, IsSource: true}
	filterComplex = []string{}
	videoMap = []string{}
	audioMap = []string{}
	buffMap = []string{}
	filterComplexOut := ""
	idx := 0

	allOutputs := []FFMpegOutput{source, p1080, p720, p360}
	for _, o := range allOutputs {
		if o.IsSource || o.Width < source.Width || o.Height < source.Height {
			filterComplexOut = filterComplexOut + fmt.Sprintf("[v%v]", idx)
			filterComplex = append(filterComplex, fmt.Sprintf("[v%v]scale=w=%v:h=%v[v%vout]", idx, o.Width, o.Height, idx))
			videoMap = append(videoMap, strings.Split(fmt.Sprintf("-map [v%vout] -c:v:%v libx265 -preset medium -crf 23 -g 60", idx, idx), " ")...)
			audioMap = append(audioMap, strings.Split(fmt.Sprintf("-map a:0 -c:a:%v aac -b:a:%v %vk", idx, idx, o.AudioBitrate), " ")...)
			buffMap = append(buffMap, fmt.Sprintf("v:%v,a:%v", idx, idx))
			idx++
		}
	}
	filterComplexMap := fmt.Sprintf("[0:v]split=%v%v", idx, filterComplexOut)
	filterComplex = slices.Concat([]string{filterComplexMap}, filterComplex)

	return
}

func RunFfmpeg(fileName string, path string, outputPath string) error {
	dimensionsArgs := []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", path}
	dimensions := exec.Command("ffprobe", dimensionsArgs...)
	dimensionsOut, dimensionsErr := dimensions.Output()
	fmt.Println(fmt.Sprintf("dimensions input: %s", dimensions.String()))
	var maxWidth string
	var maxHeight string
	if dimensionsErr != nil {
		maxWidth = "1920"
		maxHeight = "1080"
	} else {
		dimensionsArr := strings.Split(string(dimensionsOut), ",")
		maxWidth, maxHeight = dimensionsArr[0], dimensionsArr[1]
	}
	width, wErr := strconv.Atoi(strings.TrimSpace(maxWidth))
	if wErr != nil {
		return wErr
	}
	height, hErr := strconv.Atoi(strings.TrimSpace(maxHeight))
	if hErr != nil {
		return hErr
	}
	fmt.Println("dimensions:", maxWidth, maxHeight)

	filterComplex, videoMap, audioMap, buffMap := getFFMpegArgs(width, height)
	transcodeArgs := slices.Concat([]string{"-i", path, "-filter_complex", fmt.Sprintf("%v", strings.Join(filterComplex, "; "))}, videoMap, audioMap, []string{"-hls_list_size", "0", "-f", "hls", "-hls_time", "10", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments", "-hls_segment_type", "mpegts", "-hls_segment_filename", fmt.Sprintf("%s%s%s", outputPath, fileName, "%v-%03d.ts"), "-master_pl_name", fmt.Sprintf("%s.%s", fileName, "m3u8"), "-var_stream_map", fmt.Sprintf("%v", strings.Join(buffMap, " "))}, []string{fmt.Sprintf("%s%s%s", outputPath, fileName, "%v-playlist.m3u8")})
	transcode := exec.Command("ffmpeg", transcodeArgs...)

	fmt.Println(fmt.Sprintf("ffmpeg input: %s", transcode.String()))
	transcodeOut, transcodeErr := transcode.Output()
	if transcodeErr != nil {
		return fmt.Errorf("Error running ffmpeg: %s", transcodeErr.Error())
	}
	fmt.Println(fmt.Sprintf("ffmpeg output: %s", string(transcodeOut)))
	return nil
}
