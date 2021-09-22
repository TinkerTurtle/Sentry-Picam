"use strict";
var SentryPicam = function(canvasID, wsAddress){
    var topOffset = document.querySelector('#' + canvasID).offsetTop + 4;
    var ws; // websocket for video stream
    var wsMotion; // websocket for motion stream
    var player; // Broadway player
    var ticks = 0; // increments for each NAL unit
    var scalingFactor = 1; // scaling factor when canvas is resized
    var motionCanvas; // canvas for motion visualization
    var mctx; // 2d context for the motion canvas
    var coordinateCache = []; // cache to translate indicies to coordinates
    var motionMask = []; // stores areas to mask off
    var dispMotionBlockWidth = 0;
    var motionFrameWidth = 0;
    var origBgColor = document.querySelector('body').style.backgroundColor;
    var origTitle = document.title;
    var isRunningMotionMaskUx = false;
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
                    paintMotionMask();
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
            switch(typeof evt.data) {
                case "string":
                    var inc = JSON.parse(evt.data);
                    motionMask = new Int8Array(inc.mask);
                    break;
                default:
                    frame = new Uint8Array(evt.data);
                    if(motionFrameWidth != 0) {
                        highlightMotion(frame);
                    }
            }
        }
    }

    function resizeVideo() {
        var container = document.getElementById(canvasID);
        var clientWidth = document.querySelector('html').clientWidth - 1;
        var clientHeight = window.innerHeight - topOffset - 4;
        var leftOffset = -(player.canvas.width - clientWidth) / 2;

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

    function drawBoxMask(x, y) {
        mctx.beginPath();
        mctx.rect(x, y, dispMotionBlockWidth, dispMotionBlockWidth);
        mctx.strokeStyle = "gray";
        mctx.fillStyle = "rgba(0, 0, 0, 0.7)";
        mctx.fill();
        mctx.stroke();
    }

    function buildCoordinateCache() {
        var x = 0;
        var y = 0;
        var maxI = (motionCanvas.width * motionCanvas.height) / (dispMotionBlockWidth * dispMotionBlockWidth);

        for(var i = 0; i < maxI; i++) {
            coordinateCache.push({x: x, y: y});

            x += dispMotionBlockWidth;
            if(x % motionCanvas.width == 0) {
                y += dispMotionBlockWidth;
                x = 0;
            }
        }
    }

    function highlightMotion(frame) {
        for(var i in frame) {
            if(frame[i] == 1) {
                drawBox(coordinateCache[i].x, coordinateCache[i].y);
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

    var motionMaskUx = {};
    function evtMouseDown(evt) {
        motionMaskUx.isDrawing = true;
        var x = (evt.clientX - motionMaskUx.rect.left) / scalingFactor;
        var y = (evt.clientY - motionMaskUx.rect.top) / scalingFactor;
        var i = Math.floor(x / dispMotionBlockWidth) + motionFrameWidth * Math.floor(y / dispMotionBlockWidth);
        motionMaskUx.brush = motionMask[i] == 1 ? 0 : 1;
    }

    function evtMouseUp(evt) {
        var x = (evt.clientX - motionMaskUx.rect.left) / scalingFactor;
        var y = (evt.clientY - motionMaskUx.rect.top) / scalingFactor;
        var i = Math.floor(x / dispMotionBlockWidth) + motionFrameWidth * Math.floor(y / dispMotionBlockWidth);
        motionMask[i] = motionMaskUx.brush;
        motionMaskUx.isDrawing = false;
        wsMotion.send(motionMask);
    }

    function evtMouseMove(evt) {
        if(motionMaskUx.isDrawing) {
            var x = (evt.clientX - motionMaskUx.rect.left) / scalingFactor;
            var y = (evt.clientY - motionMaskUx.rect.top) / scalingFactor;
            var i = Math.floor(x / dispMotionBlockWidth) + motionFrameWidth * Math.floor(y / dispMotionBlockWidth);
            motionMask[i] = motionMaskUx.brush;
        }
    }

    function initMotionMaskUx() {
        if(coordinateCache.length == 0) {
            console.log("Coordinate cache not initialized");
            return;
        }
        if(motionMask.length == 0) {
            motionMask = new Int8Array(coordinateCache.length).fill(0);
        }

        motionMaskUx.rect = motionCanvas.getBoundingClientRect();
        motionMaskUx.isDrawing = false;
        motionMaskUx.brush = 1;
        motionCanvas.addEventListener("mousedown", evtMouseDown);
        motionCanvas.addEventListener("mouseup", evtMouseUp);
        motionCanvas.addEventListener("mousemove", evtMouseMove);
    }

    function destroyMotionMaskUx() {
        motionCanvas.removeEventListener("mousedown", evtMouseDown);
        motionCanvas.removeEventListener("mouseup", evtMouseUp);
        motionCanvas.removeEventListener("mousemove", evtMouseMove);
    }

    function toggleMotionMaskUx() {
        if(isRunningMotionMaskUx) {
            isRunningMotionMaskUx = false;
            destroyMotionMaskUx();
        }
        else {
            isRunningMotionMaskUx = true;
            initMotionMaskUx();
        }
        return isRunningMotionMaskUx;
    }

    function paintMotionMask() {
        if(isRunningMotionMaskUx) {
            motionMask.forEach(function(v, i) {
                if(v == 0) {
                    drawBoxMask(coordinateCache[i].x, coordinateCache[i].y);
                }
            });
        }
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
                buildCoordinateCache();
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
        setTopOffset: function(offset) { topOffset = offset; },
        toggleMotionMaskUx: toggleMotionMaskUx
    }
};
