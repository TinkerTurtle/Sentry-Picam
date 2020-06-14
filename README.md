# simple-webcam

Simple-webcam makes it easy to set up efficient motion detected video on Raspberry Pi hardware. Full HD 30fps video capture and motion detection on a Raspberry Pi Zero W is possible without overly taxing the CPU (~36% usage).

[Broadway](https://github.com/mbebenita/Broadway) is used to decode H264 video provided via websockets. For 1080p/30fps video, this can potentially lag behind real-time by more than the usual 300ms. Implementing WebRTC might be a better solution.

## Minimum Hardware Requirements
* Raspberry Pi Zero
* Camera Module v2 (others not tested)

## Quick Setup
```
git clone https://github.com/TinkerTurtle/simple-webcam
cd simple-webcam
./simple-webcam
```

Navigate to http://IP_address_of_your_RPi:8080


## JavaScript console commands
```
cam.startrecord()  // This starts recording video only when motion is detected
cam.stoprecord()
```

### Tips
The default video settings strike a good balance between video quality and resource usage.
To View options:
```
./simple-webcam -help
```
For max quality (bitrate depends on your network):
```
./simple-webcam -height 1088 -width 1920 -fps 30 -bitrate 4000000
```
The "Edit Detection Sectors" helps exclude motion detection in areas that you designate.
This is pretty effective at creating a bird-feeder highlight reel.
