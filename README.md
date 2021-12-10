# Sentry-Picam

![Mosaic of a 3d printed sentry turret, bird, and screenshot of a bar graph](https://raw.githubusercontent.com/TinkerTurtle/TinkerTurtle.github.io/main/img/sentry-picam.png)

Sentry-Picam is a simple wildlife / security camera solution for the Raspberry Pi Zero W, providing 1080p/30fps motion activated H.264 video capture. The built in web interface makes it easy to review video clips and identify the busiest times of day.

Motion detection in Sentry-Picam uses vectors provided by RaspiVid's video pipeline, enabling performant and effective supression of video noise.

Thanks to [Broadway](https://github.com/mbebenita/Broadway) and [RaspiVid](https://github.com/raspberrypi/userland/blob/master/host_applications/linux/apps/raspicam/RaspiVid.c), the Pi Zero W hardware can also stream live video to multiple devices with a ~300ms delay over Wifi.


## Minimum Hardware Requirements
* Raspberry Pi Zero
* Raspberry Pi Camera Module v2

## Prerequisite Software
* raspivid  - This required for motion vector data
* ffmpeg    - This is only required for custom triggers and reviewing recordings.

## Quick Setup
* Ensure camera is enabled in raspi-config
```
git clone https://github.com/TinkerTurtle/sentry-picam
cd sentry-picam
./sentry-picam
```

Navigate to http://IP_address_of_your_RPi:8080


## Tips
1. The default video settings strike a good balance between video quality and resource usage.
To View options:
    ```
    ./sentry-picam -help
    ```
2. For higher quality on Camera Module v2:
    ```
    ./sentry-picam -height 1088 -width 1920 -fps 30 -bitrate 4000000
    ```

3. Use "Edit Detection Sectors" in the web UI to specify areas where motion detection should be triggered.

4. Set up auto start:
    ```
    sudo cp sentry-picam.service /etc/systemd/system/
    sudo systemctl enable sentry-picam
    sudo systemctl start sentry-picam
    ```

5. Custom programs can be set up to trigger other functionality, like notifications or image classification. Ffmpeg is a prerequisite. 

    Sentry-picam runs your program after generating a thumbnail, and passes in the video/thumbnail name as an argument to your program. Your program will need to append the .mp4 file extension to access the video, or .jpg to access the thumbnail. Recordings are stored in ```./www/recordings/```
    ```
    ./sentry-picam -run my_script.sh
    ```

6. Files discarded from the web interface may be recovered from the folder ```./www/recordings/deleteme/```. The web interface will occasionally empty this folder, starting with recordings over 7 days old.

## Compiling from source code from Windows for a Raspberry Pi Zero
```
git clone https://github.com/TinkerTurtle/sentry-picam
cd sentry-picam

# Set environment variables for the Go compiler
SET GOOS=linux
SET GOARCH=arm
SET GOARM=6

go build
```

## STLs for the Portal Turret
https://www.thingiverse.com/thing:8277

https://www.prusaprinters.org/prints/76478-supplemental-portal-turret-components
## Enjoy!
![Cardinal swinging on a birdfeeder while eating birdfeed](https://raw.githubusercontent.com/TinkerTurtle/TinkerTurtle.github.io/main/img/cardinal.gif)
