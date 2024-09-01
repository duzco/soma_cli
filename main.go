package main

import (
	"bufio"
	"fmt"
	"encoding/xml"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
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

var stopChannel = make(chan bool)

func playStream(streamURL string) {
	resp, err := http.Get(streamURL)
	if err != nil {
		log.Fatalf("Failed to get stream: %v", err)
	}

	stream := resp.Body

	decoder, format, err := mp3.Decode(stream)
	if err != nil {
		log.Fatalf("Failed to decode mp3: %v", err)
	}

	done := make(chan bool)

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	// Play the stream in a separate goroutine
	go func() {
		speaker.Play(beep.Seq(decoder, beep.Callback(func() {
			done <- true
		})))

		for {
			select {
			case <-stopChannel:
				speaker.Clear()
				return
			default:
				// Wait for user command to switch or quit
				fmt.Print("\nEnter 'sw' to switch stations or 'Ctrl + c' to quit: ")
				command, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				command = strings.TrimSpace(command)

				if command == "sw" {
					stopStation()
					return
				}
			}
		}
	}()

	<-done
}

func stopStation() {
	stopChannel <- true
	fmt.Println("Stream stopped.")
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