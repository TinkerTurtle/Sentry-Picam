"use strict";
var Wakelock = function (callback) {
    var status = false;

    document.addEventListener('click', enableWakelock);
    document.addEventListener('touchstart', enableWakelock);

    function enableWakelock() {
        if (!status) {
            status = true;
            document.querySelector('video').play();

            document.removeEventListener('click', enableWakelock);
            document.removeEventListener('touchstart', enableWakelock);

            if(typeof callback == "function") {
                callback();
            }
        }
    }

    return {
        status: function() {
            return status;
        }
    };
}