import numpy as np
import sounddevice as sd
from scipy.fft import rfft
from pydub import AudioSegment
from pydub.playback import play
import io
import requests

# Settings
SAMPLE_RATE = 44100  # Hertz
BUFFER_SIZE = 1024   # Samples per buffer (larger sizes improve frequency resolution)
MAX_CHAR_WIDTH = 50  # Maximum number of '#' characters to display

STREAM_URL = "https://ice6.somafm.com/indiepop-128-mp3"  # SomaFM stream URL

def clear_terminal():
    """Clear the terminal screen."""
    print("\033[H\033[J", end="")

def process_audio_segment(segment):
    """Process the audio segment and display the visualizer."""
    # Convert the audio segment to raw data
    samples = np.array(segment.get_array_of_samples())
    
    # Apply FFT to the raw audio data
    fft_data = np.abs(rfft(samples))

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

def stream_audio():
    """Stream audio from the URL and process it."""
    # Stream audio from URL using requests
    response = requests.get(STREAM_URL, stream=True)
    
    # Buffer size for streaming (in bytes)
    buffer_size = BUFFER_SIZE * 4  # Adjust based on audio format (e.g., 16-bit stereo = 4 bytes per sample)

    audio_buffer = io.BytesIO()
    
    for chunk in response.iter_content(chunk_size=buffer_size):
        audio_buffer.write(chunk)
        audio_buffer.seek(0)
        
        # Convert the buffered data to an AudioSegment
        audio_segment = AudioSegment.from_file(audio_buffer, format="mp3")
        
        # Process the audio segment
        process_audio_segment(audio_segment)
        
        # Clear the buffer
        audio_buffer.seek(0)
        audio_buffer.truncate()

def main():
    print("Starting audio stream from URL...")
    stream_audio()

if __name__ == "__main__":
    main()
