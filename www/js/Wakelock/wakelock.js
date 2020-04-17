"use strict";
var Wakelock = function (domId, callback) {
    var status = false;

    document.addEventListener('click', enableWakelock);
    document.addEventListener('touchstart', enableWakelock);

    function enableWakelock() {
        if (!status) {
            status = true;
            document.querySelector('#' + domId).play();

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