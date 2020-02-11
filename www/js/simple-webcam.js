"use strict";
var SimpleWebcam = function(canvasID, wsAddress){
    var topOffset = 32;
    var ws; // websocket for video stream
    var wsMotion; // websocket for motion stream
    var player; // Broadway player
    var ticks = 0; // increments for each NAL unit
    var motionCanvas; // canvas for motion visualization
    var mctx; // 2d context for the motion canvas
    var dispMotionBlockWidth = 0;
    var motionFrameWidth = 0;
    initPlayer();
    initVideoStream()
    initMotionStream()

    function initVideoStream() {
        ws = new WebSocket(wsAddress+ "/ws/video");
        ws.binaryType = "arraybuffer";
    
        ws.onopen = function(evt) {
            ws.send("start");
        }
    
        ws.onclose = function(evt) {
            console.log("disconnected")
        }
    
        var frame;
        ws.onmessage = function(evt) {
            switch(typeof evt.data) {
                case "string":
                    handleCmd(JSON.parse(evt.data));
                    break;
                default:
                    if(evt.data.length === 0) {
                        return;
                    }
                    mctx.clearRect(0, 0, motionCanvas.width, motionCanvas.height);
                    frame = new Uint8Array(evt.data);
                    player.decode(frame);
                    ticks++;
            }
        }
    }

    function initMotionStream() {
        wsMotion = new WebSocket(wsAddress+ "/ws/motion");
        wsMotion.binaryType = "arraybuffer";
    
        wsMotion.onopen = function(evt) {
            wsMotion.send("start");
        }
    
        wsMotion.onclose = function(evt) {
            console.log("disconnected motion stream")
        }
    
        var frame;
        wsMotion.onmessage = function(evt) {
            if(evt.data.length === 0) {
                return;
            }
            frame = new Uint8Array(evt.data);
            if(motionFrameWidth != 0) {
                highlightMotion(frame);
            }
        }
    }

    function resizeVideo() {
        var container = document.getElementById(canvasID);
        var clientWidth = document.querySelector('html').clientWidth - 1;
        var clientHeight = window.innerHeight - topOffset - 4;
        var leftOffset = -(player.canvas.width - clientWidth) / 2;
        var scalingFactor = 1;
        //player.canvas.style.position = "absolute";
        container.style.position = "absolute";
        if(player.canvas.width > clientWidth) {
            scalingFactor = 1 / (player.canvas.width / clientWidth);
            container.style.transform = 'scale(' + scalingFactor + ')';
            container.style.left = leftOffset + "px";
            container.style.top = (topOffset + (leftOffset / player.canvas.width) * player.canvas.height) + "px";
        }
        if(player.canvas.height * scalingFactor > clientHeight) {
            scalingFactor = 1 / (player.canvas.height / clientHeight);
            container.style.transform = 'scale(' + scalingFactor + ')';
            container.style.top = (topOffset + -(player.canvas.height - clientHeight) / 2) + "px";
        }
    }
    window.addEventListener('resize', function() {
        resizeVideo();
    });

    function drawBox(x, y) {
        mctx.beginPath();
        mctx.rect(x, y, dispMotionBlockWidth, dispMotionBlockWidth);
        mctx.strokeStyle = "red";
        mctx.stroke();
    }

    function highlightMotion(frame) {
        var x = 0;
        var y = 0;

        for(var i in frame) {
            if(frame[i] == 1) {
                drawBox(x, y);
            }

            x += dispMotionBlockWidth;
            if(x % motionCanvas.width == 0) {
                y += dispMotionBlockWidth;
                x = 0;
            }
        }
    }

    function initPlayer() {
        player = new Player({
            webgl: "auto",
            useWorker: true,
            workerFile: './js/Broadway/Decoder.js'
        });
    }

    function initCanvas(msg) {
        player.canvas.width = msg.width;
        player.canvas.height = msg.height;
        resizeVideo();

        document.getElementById(canvasID).appendChild(player.canvas);

        motionCanvas = document.createElement('canvas');
        motionCanvas.width = msg.width;
        motionCanvas.height = msg.height;
        motionCanvas.style.position = "absolute";
        motionCanvas.style.left = "0px";
        motionCanvas.style.top = "0px";
        document.getElementById(canvasID).appendChild(motionCanvas);
        mctx = motionCanvas.getContext("2d");
    }

    function handleCmd(msg) {
        switch(msg.action) {
            case "init":
                initCanvas(msg);
                motionFrameWidth = (msg.width / 16) / msg.mbWidth;
                dispMotionBlockWidth = msg.width / motionFrameWidth;
                break;
            default:
                console.log(msg);
        }
    }

    function set(msg) {
        switch(msg.mode) {
            case "DAY":
                ws.send("mode:day");
                break;
            case "NIGHT":
                ws.send("mode:night");
                break;
        }
    }

    return {
        startrecord: function() { ws.send('startrecord'); },
        stoprecord: function() { ws.send('stoprecord'); },
        stop: function() { ws.close(); wsMotion.close(); },
        getTick: function() { return ticks; },
        set: set,
        setTopOffset: function(offset) { topOffset = offset; }
    }
};
