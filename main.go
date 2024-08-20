package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"bufio"
	"strings"
	"encoding/xml"
	"net/http"
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

var cmd *exec.Cmd

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
##############################################################.......#############
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
				selectedStationName = name
				selectedStation = &station
				break
			}
		}

		// If no shortcut matched, check if the input matches a station name
		if selectedStation == nil {
			if station, exists := stations[input]; exists {
				selectedStationName = input
				selectedStation = &station
			}
		}

		if selectedStation == nil {
			fmt.Println("Invalid station name or shortcut. Please try again.")
			continue
		}
		
		// Start the ffplay process with fade-in and volume control
		cmd = exec.Command("ffplay", "-vn", "-nodisp", "-af", "afade=t=in:st=0:d=3,volume=1.0", selectedStation.URL)

		// Start the process in the background
		err := cmd.Start()
		if err != nil {
			log.Fatalf("Error starting ffplay: %v", err)
		}


		// Fetch and print the track data for the selected station
		fetchAndPrintTrackData(selectedStation.SongsURL, selectedStationName)

		// Wait for user command to switch or quit
		go handleSignals(cmd)

		for {
			fmt.Print("\nEnter 'sw' to switch stations or 'Ctrl + c' to quit: ")
			command, _ := reader.ReadString('\n')
			command = strings.TrimSpace(command)

			if command == "sw" {
				stopStation()
				break
			}
		}
	}
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

	//fmt.Println("\nCurrent Track Data:")

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

// Function to stop the current ffplay process
func stopStation() {
	fmt.Println("Stopping the current station...")
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	fmt.Println("Stream stopped.")
}

// Handle interrupt signals like Ctrl+C
func handleSignals(cmd *exec.Cmd) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	stopStation()
	os.Exit(0)
}