var ip = require('ip');
var dgram = require('dgram');
var crypto = require('crypto');
var natpmp = require('nat-pmp');
var keypair = require('keypair');
var natupnp = require('nat-upnp');

var Net = require('./network.js');
var Utils = require('./utils.js');

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
    verified: false,
    routingTableBuilt: false,
    bootstrapPeer: null,
}

var net = new Net(globalConfig);
var utils = net.utils;

var stdin = process.openStdin();
globalConfig['peerTable'] = new Array(256);
globalConfig['keyTable'] = {};

console.log('generating keypair...');
var pair = keypair({bits: 2048}); 
var id = utils.sha256(pair.public);
console.log(`id: ${id}`);

console.log('generating ephemeral keys...');
// var dh = crypto.createDiffieHellman(1024);
// var pubkey = dh.generateKeys('hex');
console.log(`initializing server...`);

globalConfig['id'] = id;
globalConfig['pair'] = pair;
// globalConfig['dh'] = dh;

globalConfig['idealRoutingTable'] = utils.calculateIdealRoutingTable(globalConfig['id']);
globalConfig['peerCandidates'] = [];
globalConfig['chatMessages'] = new Array(512);
globalConfig['chatMessagesReceived'] = {};

// TODO: create image with fingerprint

// console.log('generating keys...');
// var dh2 = crypto.createDiffieHellman(dh.getPrime(), dh.getGenerator());
// var pubKey2 = dh2.generateKeys('hex');
// console.log(`pubkey ${pubKey2}`);
// // id is sha512 of public key
// var fingerprint2 = crypto.createHash('sha256').update(Buffer.from(pubKey2, 'hex')).digest('hex');
// console.log(`fingerprint: ${fingerprint2}`);

// dh2.computeSecret(pubKey, 'hex');
// var sessionSecret = dh.computeSecret(pubKey2, 'hex');

function bootstrap(toks) {
    var pair = globalConfig['pair'];

    if(toks.length < 2) {
        return false;
    }

    var conn = toks[1];
    var match = conn.match(/([a-fA-F0-9]{8}):([a-fA-F0-9]{4}):([a-fA-F0-9]{64})/);
    if(!match) {
        return false;
    }

    var ipEnc = match[1];
    var portEnc = match[2];
    var peerId = match[3];

    var ip = [
        parseInt(ipEnc.substr(0, 2), 16),
        parseInt(ipEnc.substr(2, 2), 16),
        parseInt(ipEnc.substr(4, 2), 16),
        parseInt(ipEnc.substr(6, 2), 16),
    ].map(function(ea) {
        return ea.toString();
    }).join('.');

    var port = parseInt(portEnc, 16);
    
    globalConfig['bootstrapPeer'] = {ip, port, id: peerId};
    console.log('bootstrapping to', globalConfig['bootstrapPeer']);

    net.join(pair, ip, port, peerId);

    return true;
}

function dumpPeerTable() {
    console.log(globalConfig['peerTable'][0]);
    console.log(globalConfig['peerTable'][128]);
    console.log(globalConfig['peerTable'][255]);
    return true;
}

function dumpKeyTable() {
    console.log(globalConfig['keyTable']);
    return true;
}

function handleCommand(command) {
    var toks = command.split(' ');
    var cmd = toks[0];
    
    switch(cmd) {
        case '/bootstrap':
        case '/bs':
        case '/join':
            return bootstrap(toks);
        case '/peerTable':
        case '/pt':
            return dumpPeerTable();
        case '/keyTable':
        case '/kt':
            return dumpKeyTable();
        case '/leave':
        case '/exit':
        case '/quit':
            return net.leave(pair);
        default:
            break;
    }
    
    // kill attempts to message before initialized
    if(!globalConfig['verified'] || !globalConfig['routingTableBuilt'])
        return true;

    return false;
}

function serverInit() {
    console.log('setting up socket...');
    socket.on('error', (err) => {
        console.error(`server error:\n${err.stack}`);
        socket.close();
        process.exit(1);
    });

    var pair = globalConfig['pair'];

    socket.on('message', (msgJSON, rinfo) => {
        // handle external message
        if(!utils.validateMsg(msgJSON, ['type'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        switch(data['type']) {
            case 'join':
                net.onJoin(pair, msgJSON);
                break;
            case 'verify':
                net.onVerify(pair, msgJSON);
                break;
            case 'leave':
                net.onLeave(pair, msgJSON);
                break;
            case 'announce':
                net.onAnnounce(pair, msgJSON);
                break;
            case 'query_closest':
                net.onQueryClosest(pair, msgJSON);
                break;
            case 'response_closest':
                net.onResponseClosest(pair, msgJSON);
                break;
            case 'chat':
                net.onChat(msgJSON);
                break;
            default:
                break;
        }
    });

    socket.on('listening', () => {
        const address = socket.address();
    });

    socket.bind(globalConfig['int']['port']);

    globalConfig['socket'] = socket;

    var bootstrapInfo = globalConfig['ext']['ip'].split('.').map(function(ea) {
        return parseInt(ea).toString(16).padStart(2, '0');
    }).join('');

    bootstrapInfo += ':';
    bootstrapInfo += globalConfig['ext']['port'].toString(16).padStart(4, '0');
    bootstrapInfo += ':';
    bootstrapInfo += globalConfig['id'];

    console.log(`bootstrap info: ${bootstrapInfo}`);
    globalConfig['bootstrapInfo'] = bootstrapInfo;

    stdin.addListener("data", function(d) {
        var input = d.toString().trim();
        if(!handleCommand(input)) {
            // handle userInput
            var data = {
                type: 'chat',
                id: globalConfig['id'],
                ts: Date.now(),
                content: input,
            }
            // utils.addChat(data);
            net.flood(globalConfig['pair'], data);
        }
    });
    console.log(`bootstrap to a peer or have a peer bootstrap to you to get started`);
}

function killError(err) {
    if(err) {
        console.error(err);
        process.exit(1);
    }
}

function getIp() {
    upnpClient.externalIp(function(err, ip) {
        if(err) {
            killError(err);
        }
        globalConfig['ext']['ip'] = ip;
        console.log(ip);
        serverInit();
    });
}

function holePunch(results) {
    // find unused port
    var udpResults = results.slice(0,20).filter(function(ea) { 
        return ea['protocol'] === 'udp';
    });

    var localResults = results.filter(function(ea) {
        return ea['private']['host'] === globalConfig['int']['ip'];
    });

    var externalPorts = udpResults.map(function(ea) {
        return ea['public']['port'];
    });

    var internalPorts = localResults.map(function(ea) {
        return ea['private']['port'];
    });

    var topPicks = [0x1337, 0xbeef, 0xdab, 0xbea7, 0xf00d, 0xc0de, 0x0bee, 0xdead, 0xbad, 0xdab0, 0xbee5, 0x539];
    var picks = topPicks.filter(function(ea) {
        return !(ea in internalPorts) && !(ea in externalPorts);
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
        console.log('mapped port...');
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
    if(ip.address().match(/(^127\.)|(^192\.168\.)|(^10\.)|(^172\.1[6-9]\.)|(^172\.2[0-9]\.)|(^172\.3[0-1]\.)|(^::1$)|(^[fF][cCdD])/)) {
        console.log('attempting upnp...')
        upnpClient.getMappings(function(err, results) {
            killError(err);
            var found = false;
            for(ea in results) {
                var description = results[ea]['description'];
                var privateIp = results[ea]['private']['host'];
                var enabled = results[ea]['enabled'];
                var udp = results[ea]['protocol'] === 'udp';

                if(enabled && udp && description == 'Party line!' &&  privateIp == globalConfig['int']['ip']) {
                    found = true;
                    console.log('found already open port');
                    globalConfig['int']['port'] = results[ea]['private']['port'];
                    globalConfig['ext']['port'] = results[ea]['public']['port'];
                    break;
                } 
            } 

            if(!found) {
                console.log('open port not found, mapping');
                holePunch(results);
                return;
            }
            
            getIp();
        });
    } else {
        globalConfig['int']['port'] = 0xdab;
        globalConfig['ext']['port'] = 0xdab;
        globalConfig['int']['ip'] = ip.address();
        globalConfig['ext']['ip'] = ip.address();
        serverInit();
    }
    // nat-pmp when the need arises, upnp seems to work everywhere I've tried
}

function listMappings() {
    upnpClient.getMappings(function(err, results) {
        killError(err);
        for(ea in results) {
            var description = results[ea]['description'];
            var privateIp = results[ea]['private']['host'];
            var udp = results[ea]['protocol'] === 'udp';

            if(udp && description == 'Party line!') {
                console.log(results[ea]);
            } 
        } 
    });
}

init();
// listMappings();
