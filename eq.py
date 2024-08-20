import numpy as np
import sounddevice as sd
from scipy.fft import rfft

# Settings
SAMPLE_RATE = 44100  # Hertz
BUFFER_SIZE = 1024   # Samples per buffer (larger sizes improve frequency resolution)
MAX_CHAR_WIDTH = 50  # Maximum number of '#' characters to display

def clear_terminal():
    """Clear the terminal screen."""
    print("\033[H\033[J", end="")

def audio_callback(indata, frames, time, status):
    if status:
        print(status)

    # Apply FFT to the input audio data
    fft_data = np.abs(rfft(indata[:, 0]))  # Apply FFT to the first channel (mono)

    # Divide into 10 bands
    num_bins = len(fft_data)
    bands = np.array_split(fft_data, 10)

    # Clear terminal
    clear_terminal()

    # Calculate average magnitude for each band and print the row of '#'
    for i, band in enumerate(bands):
        avg_magnitude = np.mean(band)
        
        # Scale the magnitude to fit within the MAX_CHAR_WIDTH
        num_hashes = min(MAX_CHAR_WIDTH, int(avg_magnitude / np.max(fft_data) * MAX_CHAR_WIDTH))
        
        # Generate and print the row of '#'
        band_row = "#" * num_hashes
        start_freq = i * (SAMPLE_RATE / 2 / 10)
        end_freq = (i + 1) * (SAMPLE_RATE / 2 / 10)
        print(f"Band {i+1} ({int(start_freq)}-{int(end_freq)} Hz): {band_row}")

def main():
    print("Starting audio stream...")
    with sd.InputStream(callback=audio_callback, channels=1, samplerate=SAMPLE_RATE, blocksize=BUFFER_SIZE):
        print("Stream is running...")
        while True:
            sd.sleep(100)  # Keep the stream running

if __name__ == "__main__":
    main()
