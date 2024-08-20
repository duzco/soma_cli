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
)

type Station struct {
	URL      string
	Shortcut rune
}

var cmd *exec.Cmd

// Station data: station name -> stream URL
var stations = map[string]Station{
	"Lush":         				{URL:"https://ice6.somafm.com/lush-128-mp3", Shortcut:'l'},
	"Groove Salad": 				{URL:"https://ice6.somafm.com/groovesalad-128-mp3", Shortcut: 'g'},
	"Indie Pop Rocks!": 		{URL:"https://ice6.somafm.com/indiepop-128-mp3", Shortcut: 'i'},
	"Secret Agent": 				{URL:"https://ice6.somafm.com/secretagent-128-mp3", Shortcut: 's'},
	"Underground 80s": 			{URL:"https://ice6.somafm.com/u80s-128-mp3", Shortcut: '8'},
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
		input = strings.TrimSpace(input) // Remove newline and spaces

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

		fmt.Printf("Playing %s Station. Press 'switch' to change stations or 'q' to quit.\n", selectedStationName)

		// Wait for user command to switch or quit
		go handleSignals(cmd)

		for {
			fmt.Print("Enter 'sw' to switch stations or 'Ctrl + c' to quit: ")
			command, _ := reader.ReadString('\n')
			command = strings.TrimSpace(command)

			if command == "sw" {
				stopStation()
				break
			}
		}
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