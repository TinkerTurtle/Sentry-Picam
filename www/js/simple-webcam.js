var SimpleWebcam = function(canvasID, wsAddress){
    var topOffset = 32;
    var player; // Broadway player
    var ticks = 0; // increments for each NAL unit
    initPlayer();

    var ws = new WebSocket(wsAddress);
    ws.binaryType = "arraybuffer";

    ws.onopen = function(evt) {
        ws.send("REQUESTSTREAM");
    }

    ws.onclose = function(evt) {
        console.log("disconnected")
    }

    ws.onmessage = function(evt) {
        switch(typeof evt.data) {
            case "string":
                handleCmd(JSON.parse(evt.data));
                break;
            default:
                if(evt.data.length === 0) {
                    return;
                }
                frame = new Uint8Array(evt.data);
                player.decode(frame);
                ticks++;
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

    function initPlayer() {
        player = new Player({
            webgl: "auto",
            useWorker: true,
            workerFile: './js/Broadway/Decoder.js'
        });

        document.getElementById(canvasID).appendChild(player.canvas);
    }

    function initCanvas(msg) {
        player.canvas.width = msg.width;
        player.canvas.height = msg.height;
        resizeVideo();
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
                ws.send("DAYMODE");
                break;
            case "NIGHT":
                ws.send("NIGHTMODE");
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
