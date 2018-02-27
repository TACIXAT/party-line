var nacl_factory = require('js-nacl');
var natpmp = require('nat-pmp');
var natupnp = require('nat-upnp');
var async = require('async');
var dgram = require('dgram');
var ip = require('ip');

var upnpClient = natupnp.createClient();
var socket = dgram.createSocket('udp4');

var globalConfig = {
    int: {
        ip: ip.address(),
        port: null,
    }, 
    ext: {
        ip: null,
        port: null,
    },
}

var stdin = process.openStdin();
// peer {ip: '6.6.6.6', port: 0x1337}
var peers = [];

function bootstrap(toks) {
    if(toks.length < 2) {
        return false;
    }

    var conn = toks[1];
    var match = conn.match(/([a-fA-F0-9]{8}):([a-fA-F0-9]{4})/);
    if(!match) {
        return false;
    }

    var ipEnc = match[1];
    var portEnc = match[2];
    var ip = [
        parseInt(ipEnc.substr(0, 2), 16),
        parseInt(ipEnc.substr(2, 2), 16),
        parseInt(ipEnc.substr(4, 2), 16),
        parseInt(ipEnc.substr(6, 2), 16),
    ].map(function(ea) {
        return ea.toString();
    }).join('.');

    var port = parseInt(portEnc, 16);
    peers.push({ip: ip, port: port});
    return true;
}

function handleCommand(command) {
    var toks = command.split(' ');
    var cmd = toks[0];
    
    switch(cmd) {
        case '/bootstrap':
        case '/bs':
            return bootstrap(toks);
        default:
            return false;
    }
}

function serverInit() {
    socket.on('error', (err) => {
        console.error(`server error:\n${err.stack}`);
        socket.close();
        process.exit(1);
    });

    socket.on('message', (msg, rinfo) => {
        console.log(`server got: ${msg} from ${rinfo.address}:${rinfo.port}`);
        // handle external message
    });

    socket.on('listening', () => {
        const address = socket.address();
        console.log(`server listening ${address.address}:${address.port}`);
    });

    socket.bind(globalConfig['int']['port']);

    var bootstrapInfo = globalConfig['ext']['ip'].split('.').map(function(ea) {
        return parseInt(ea).toString(16).padStart(2, '0');
    }).join('');

    bootstrapInfo += ':';
    bootstrapInfo += globalConfig['ext']['port'].toString(16).padStart(4, '0');

    console.log(`bootstrap info: ${bootstrapInfo}`);

    stdin.addListener("data", function(d) {
        // note:  d is an object, and when converted to a string it will
        // end with a linefeed.  so we (rather crudely) account for that  
        // with toString() and then trim() 
        var input = d.toString().trim();
        console.log(`you entered: [${input}]`);

        if(!handleCommand(input)) {
            // handle userInput
            for(idx in peers) {
                var peer = peers[idx];
                socket.send(input, peer['port'], peer['ip']);
            }
        }
    });
}

function killError(err) {
    if(err) {
        console.error(err);
        process.exit(1);
    }
}

function getIP() {
    upnpClient.externalIp(function(err, ip) {
        if(err) {
            killError(err);
        }
        globalConfig['ext']['ip'] = ip;
        console.log(ip);
        serverInit();
    });
}

function map(results) {
    // find unused port
    var udpResults = results.reduce(function(acc, ea) { 
        if(ea['protocol'] === 'udp')
            return acc + ea;
        return acc;
    });

    var localResults = results.reduce(function(acc, ea) {
        if(ea['private']['host'] === globalConfig['int']['ip']) {
            return acc + ea;
        }
        return acc;
    });

    var externalPorts = udpResults.map(function(ea) {
        return ea['public']['port'];
    });

    var internalPorts = localResults.map(function(ea) {
        return ea['private']['port'];
    });

    var topPicks = [0x1337, 0xbeef, 0xdab, 0xbea7, 0xf00d, 0xc0de, 0x0bee, 0xdead, 0xbad, 0xdab0, 0xbee5, 0x539];
    var picks = topPicks.reduce(function(acc, ea) {
        if(!(ea in internalPorts) && !(ea in externalPorts)) {
            return acc + ea;
        }

        return acc;
    });

    if(picks.length > 0) {
        var port = picks[Math.floor(Math.random()*picks.length)];
        globalConfig['ext']['port'] = port;
        globalConfig['int']['port'] = port;
    } else {
        while(1) {
            var port = Math.floor(Math.random()*(65536-1025)) + 1025;
            if(!(port in externalPorts) && !(port in internalPorts)) {
                globalConfig['ext']['port'] = port;
                globalConfig['int']['port'] = port;    
                break;
            }
        }
    }

    console.log('chose port', port);

    upnpClient.portMapping({
        public: globalConfig['ext']['port'],
        private: globalConfig['int']['port'],
        ttl: 0,
        protocol: 'UDP',
        description: 'Party line!',
        local: false,
    }, function(err) {
        killError(err);
        console.log('mapped');
        init();
    });
}

function unmap() {
    upnpClient.portUnmapping({
        public: 0x1337,
        protocol: 'UDP',
        ttl: 0,
    }, function(err) {
        killError(err);
        console.log('unmapped');
    });
}

function init() {
    upnpClient.getMappings(function(err, results) {
        killError(err);
        var found = false;
        var udpPorts = [];
        for(ea in results) {
            var description = results[ea]['description'];
            var privateIP = results[ea]['private']['host'];
            var enabled = results[ea]['enabled'];
            var udp = results[ea]['protocol'] === 'udp';

            if(enabled && udp && description == 'Party line!' &&  privateIP == globalConfig['int']['ip']) {
                found = true;
                console.log('found');
                globalConfig['int']['port'] = results[ea]['private']['port'];
                globalConfig['ext']['port'] = results[ea]['public']['port'];
                break;
            } 
        } 

        if(!found) {
            console.log('not found, mapping');
            map(results);
            return;
        }
        
        getIP();
    })
}

init();

