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
        height: '100%-2',
        style: {
            fg: 'white',
            bg: 'magenta',
        },
    });

    var status = blessed.box({
        parent: screen,
        bottom: 0,
        left: 0,
        width: '100%',
        height: "0%+1",
        style: {
            bg: 'blue',
            fg: 'white',
        }
    });

    var input = blessed.textarea({
        parent: screen,
        inputOnFocus: true,
        input: true,
        keys: true,
        bottom: 1,
        left: 0,
        width: '100%',
        height: "0%+1",
        style: {
            bg: 'white',
            fg: 'black',
        }
    });

    input.focus();

    log.key('f4', function(ch, key) {
        process.exit();
    });

    input.key('f4', function(ch, key) {
        process.exit();
    });

    log.on('focus', function() {
        input.focus();
    });

    module.input = input;

    module.setEnterCallback = function(cb) {
        input.key('enter', cb);
    }

    module.logMsg = function(msg) {
        log.add(msg);
    }

    module.setStatus = function(msg) {
        status.setContent(msg);
    }

    module.start = function() {
        screen.render();
    }

    module.stop = function() {
        screen.destroy();
    }

    module.setStatus('exit (F4)')

    return module;
}