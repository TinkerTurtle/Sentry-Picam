"use strict";
var SimpleWebcam = function(canvasID, wsAddress){
    var topOffset = 32;
    var ws; // websocket for video stream
    var player; // Broadway player
    var ticks = 0; // increments for each NAL unit
    var motionCanvas; // canvas for motion visualization
    var mctx; // 2d context for the motion canvas
    initPlayer();
    initVideoStream()

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
                    highlightMotion(20, 20, 16, 16);
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

    function highlightMotion(x, y, width, height) {
        mctx.beginPath();
        mctx.rect(x, y, width, height);
        mctx.strokeStyle = "red";
        mctx.stroke();
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
        stop: function() { ws.close(); },
        getTick: function() { return ticks; },
        set: set,
        setTopOffset: function(offset) { topOffset = offset; }
    }
};
