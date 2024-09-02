package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"log"
	"math/cmplx"
	"net/http"
	"os"
	"strings"
	//"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/mjibson/go-dsp/fft"
	//"github.com/eapache/queue"
	"golang.org/x/net/html/charset"
)

type Station struct {
	URL      string
	Shortcut rune
	SongsURL string // URL for fetching song data
}

// Song structure to map the XML data
type Song struct {
	Title  string `xml:"title"`
	Artist string `xml:"artist"`
	Album  string `xml:"album"`
}

// Songs struct to wrap a list of Song elements
type Songs struct {
	Songs []Song `xml:"song"`
}

// Station data: station name -> stream URL and Songs URL
var stations = map[string]Station{
	"Lush":           {URL: "https://ice6.somafm.com/lush-128-mp3", Shortcut: 'l', SongsURL: "https://somafm.com/songs/lush.xml"},
	"Groove Salad":   {URL: "https://ice6.somafm.com/groovesalad-128-mp3", Shortcut: 'g', SongsURL: "https://somafm.com/songs/groovesalad.xml"},
	"Indie Pop Rocks!": {URL: "https://ice6.somafm.com/indiepop-128-mp3", Shortcut: 'i', SongsURL: "https://somafm.com/songs/indiepop.xml"},
	"Secret Agent":   {URL: "https://ice6.somafm.com/secretagent-128-mp3", Shortcut: 's', SongsURL: "https://somafm.com/songs/secretagent.xml"},
	"Underground 80s": {URL: "https://ice6.somafm.com/u80s-128-mp3", Shortcut: '8', SongsURL: "https://somafm.com/songs/u80s.xml"},
}

var stopChannel = make(chan bool)

func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\033[H\033[2J") // Clear the screen

		fmt.Print(`
     ########################################################################
 ###############################################################.....############ 
##############################################################...#################
##############################################################..##################
##..........###........####...........####........##########.......###..........##
#...######..##...#####...##...#...-#..###########..###########..######...#...-#..#
##-.....#####-..######...##..##...##..####.........###########..######..##...##..#
######.....##-..######...##..##...##..##...........###########..######..##...##..#
##. ##### ..#-..######...##..##...##..##..#######..###########..######..##...##..#
#...........##..........###..##...##..##...........###########..######..##...##..#
###.......######-....-#####..###.####.####.....-#..###########..######..###.####.#
 ################################################################################
    #########################################################################
		`)

		fmt.Println("\nAvailable Stations:\n(enter code or 'Ctrl + c' to quit)")
		for name, station := range stations {
			fmt.Printf("%c -- %s\n", station.Shortcut, name)
		}

		// Get user input
		fmt.Print("\nEnter the station code or name to play: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input) // Remove leading/trailing whitespaces

		// Determine the selected station
		var selectedStationName string
		var selectedStation *Station

		// Check if input matches a shortcut
		for name, station := range stations {
			if len(input) == 1 && rune(input[0]) == station.Shortcut {
				selectedStation = &station
				selectedStationName = name // Store the station name
				break
			}
		}

		// If no shortcut matched, check if the input matches a station name
		if selectedStation == nil {
			if station, exists := stations[input]; exists {
				selectedStation = &station
				selectedStationName = input // Store the station name
			}
		}

		if selectedStation == nil {
			fmt.Println("Invalid station name or shortcut. Please try again.")
			continue
		}

		// Fetch and print track data
		fetchAndPrintTrackData(selectedStation.SongsURL, selectedStationName)

		// Start playing the stream using the oto package
		playStream(selectedStation.URL)
	}
}

var (
	stopPlayback = make(chan bool)
	stopAnalysis = make(chan bool)
)

func playStream(streamURL string) {
	resp, err := http.Get(streamURL)
	if err != nil {
		log.Fatalf("Failed to get stream: %v", err)
	}
	defer resp.Body.Close()

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		log.Fatalf("Failed to decode mp3: %v", err)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)

	// Create a custom streamer that analyzes the frequencies
	streamerWithAnalysis := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		n, ok = streamer.Stream(samples)
		if !ok {
			return n, ok
		}

		// Analyze the left channel
		leftChannel := make([]float64, n)
		for i := 0; i < n; i++ {
			leftChannel[i] = samples[i][0]
		}

		go analyzeFrequencies(leftChannel)

		return n, ok
	})

	speaker.Play(beep.Seq(streamerWithAnalysis, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish or for a stop signal
	select {
	case <-done:
	case <-stopPlayback:
		speaker.Clear()
	}
}

func analyzeFrequencies(leftChannel []float64) {
	bands := calculateBands(leftChannel)
	fmt.Println("Frequency Bands:", bands)
}

func calculateBands(samples []float64) []float64 {
	fftResult := fft.FFTReal(samples)
	numBands := 10
	bandWidth := len(fftResult) / numBands
	bands := make([]float64, numBands)

	for i := 0; i < numBands; i++ {
		bandPower := 0.0
		for j := 0; j < bandWidth; j++ {
			bandPower += cmplx.Abs(fftResult[i*bandWidth+j])
		}
		bands[i] = bandPower / float64(bandWidth)
	}

	return bands
}

func fetchAndPrintTrackData(songsURL string, selectedStationName string) {
	resp, err := http.Get(songsURL)
	if err != nil {
		fmt.Println("Error fetching track data:", err)
		return
	}
	defer resp.Body.Close()

	// Create a new XML decoder with charset support
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel // Adds support for non-UTF-8 charsets

	var songs Songs
	if err := decoder.Decode(&songs); err != nil {
		fmt.Println("Error parsing track data:", err)
		return
	}

	// Print the current track (first track)
	if len(songs.Songs) > 0 {
		firstSong := songs.Songs[0]
		fmt.Println("Now playing on", selectedStationName)
		fmt.Println("Track:                               Artist:               Album:")
		fmt.Printf("%-35.35s  %-20.20s  %-20.20s\n\n", firstSong.Title, firstSong.Artist, firstSong.Album)
	}

	fmt.Println("    ----------------------------History------------------------------")
	// Print the other tracks
	for _, song := range songs.Songs[1:6] { // Five most recent tracks
		fmt.Printf("%-35.35s  %-20.20s  %-20.20s\n", song.Title, song.Artist, song.Album)
	}
}