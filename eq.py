import numpy as np
import sounddevice as sd
import matplotlib.pyplot as plt
from scipy.fft import rfft, rfftfreq

# Define audio stream parameters
sample_rate = 44100  # Standard sampling rate for audio
buffer_size = 1024   # Buffer size for each audio block

# Audio callback function for the stream
def audio_callback(indata, frames, time, status):
    if status:
        print(status)

    # Apply FFT to the audio buffer
    fft_result = rfft(indata[:, 0])
    magnitudes = np.abs(fft_result)

    # Generate frequencies corresponding to the FFT result
    freqs = rfftfreq(buffer_size, 1 / sample_rate)

    # Print some of the frequency and magnitude pairs for debugging
    for freq, magnitude in zip(freqs[:10], magnitudes[:10]):  # Limit to first 10 frequencies
        print(f"Frequency: {freq:.2f} Hz, Magnitude: {magnitude:.6f}")

def main():
    # Initialize the audio stream
    print("Starting audio stream...")
    with sd.InputStream(channels=1, callback=audio_callback, blocksize=buffer_size, samplerate=sample_rate):
        print("Stream is running...")

        # Keep the stream open
        while True:
            pass

if __name__ == "__main__":
    main()
