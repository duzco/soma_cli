/*package main

import (
	"fmt"
	"net/http"
	"github.com/mjibson/go-dsp/fft"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
)

const StreamURL = "https://ice6.somafm.com/indiepop-128-mp3"

func main() {
	resp, err := http.Get(StreamURL)
	if err != nil {
		fmt.Printf("Error accessing stream: %v\n", err)
		return
	}
	defer resp.Body.Close()

	decoder, _, err := mp3.Decode(resp.Body)
	if err != nil {
		fmt.Printf("Error decoding stream: %v\n", err)
		return
	}

	samples := make([][2]float64, 4096)

	for {
		n, ok := decoder.Stream(samples)
		if !ok || n == 0 {
			break
		}

		leftChannel := make([]float64, n)
		for i := 0; i < n; i++ {
			leftChannel[i] = samples[i][0]
		}

		bands := calculateBands(leftChannel)
		fmt.Println("Frequency Bands:", bands)
	}
}

// Calculate frequency bands using FFT
func calculateBands(samples []float64) []float64 {
	fftResult := fft.FFTReal(samples)
	numBands := 10
	bandWidth := len(fftResult) / numBands
	bands := make([]float64, numBands)

	for i := 0; i < numBands; i++ {
		bandPower := 0.0
		for j := 0; j < bandWidth; j++ {
			bandPower += real(fftResult[i*bandWidth+j])
		}
		bands[i] = bandPower / float64(bandWidth)
	}

	return bands
}
