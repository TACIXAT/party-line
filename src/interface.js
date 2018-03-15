var blessed = require('blessed');

module.exports = function() {
    var screen = blessed.screen({
        smartCSR: true,
    });

    var log = blessed.log({
        parent: screen,
        top: 0,
        left: 0,
        width: '100%',
        height: '100%-4',
        border: {
            type: 'line'
        },
        style: {
            fg: 'white',
            bg: 'magenta',
            border: {
                fg: '#f0f0f0'
            },
        },
        scrollable: true,
    });

    log.key('escape', function(ch, key) {
        process.exit();
    });

    var status = blessed.box({
        parent: screen,
        bottom: 0,
        left: 0,
        width: '100%',
        height: "0%+1",
        style: {
            bg: 'blue',
            fg: 'red',
        }
    });

    var input = blessed.textarea({
        parent: screen,
        inputOnFocus: true,
        keys: true,
        bottom: 1,
        left: 0,
        width: '100%',
        height: "0%+3",
        border: {
            type: 'line'
        },
        style: {
            bg: 'white',
            fg: 'black',
            border: {
                fg: '#f0f0f0'
            },
        }
    });

    input.focus()

    input.key('escape', function(ch, key) {
        process.exit();
    });

    module.input = input;

    module.setEnterCallback = function(cb) {
        input.key('enter', cb);
    }

    module.logMsg = function(msg) {
        log.pushLine(msg);
    }

    module.setStatus = function(msg) {
        status.setContent(msg);
    }

    module.start = function() {
        screen.render();
    }

    return module;
}