var natpmp = require('nat-pmp');
var natupnp = require('nat-upnp');
var async = require('async');
var dgram = require('dgram');
var ip = require('ip');
var crypto = require('crypto');

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

console.log('generating keys...');
var dh = crypto.createDiffieHellman(2048);
var pubKey = dh.generateKeys('hex');
console.log(`pubkey ${pubKey}`);
// id is sha512 of public key
var fingerprint = crypto.createHash('sha256').update(Buffer.from(pubKey, 'hex')).digest('hex');
console.log(`fingerprint: ${fingerprint}`);

bunction bufferXor(a, b) {
    var length = Math.max(a.length, b.length)
    var buffer = Buffer.allocUnsafe(length)

    for (var i = 0; i < length; ++i) {
        buffer[i] = a[i] ^ b[i]
    }

    return buffer
}

// Buffer.compare(b1, b2)
// -1 first is less
// 1 second is less

function calculateIdealRoutingTable(fingerprint) {
    // iterate idx over 0 to 255
    // xor fingerprint with 2**idx
    // insert ideal to position in routing table
}

var idealRoutingTable = calculateIdealRoutingTable(fingerprint);

// console.log('generating keys...');
// var dh2 = crypto.createDiffieHellman(dh.getPrime(), dh.getGenerator());
// var pubKey2 = dh2.generateKeys('hex');
// console.log(`pubkey ${pubKey2}`);
// // id is sha512 of public key
// var fingerprint2 = crypto.createHash('sha256').update(Buffer.from(pubKey2, 'hex')).digest('hex');
// console.log(`fingerprint: ${fingerprint2}`);

// dh2.computeSecret(pubKey, 'hex');
// var sessionSecret = dh.computeSecret(pubKey2, 'hex');

// TODO: create image with fingerprint

// join
function join(ip, port) {
    // connect to peer
    // get peerId
    // get peer public key
    // add peer to routing table with key
    // return peerId
}

// leave
function leave() {
    // iterate peer list
    // notify peers that node is disconnecting
}

// copy routing table
function copyRoutingTable(peerId) {
    // ask peer for routing table
    // check signature
    // return peerRoutingTable
}

// build node routing table
function buildRoutingTable(peerTable) {
    // iterate over ideal routing table
        // find closest peer in peer's routing table
        // iteratively query peers for closest to ideal
            // stop when contacted peer returns self

    // TODO: try multiple routes?
    // TODO: spam all peers for closest and work till consensus?
}

// find closest
function queryClosest(peerId, targetId) {
    // send message asking for closest
}

function addPeer(ip, port) {
    var filtered = peers.filter(function(ea) {
        return ea['ip'] == ip && ea['port'] == port;
    });

    if(filtered.length > 0) {
        return;
    }

    var peer = {ip: ip, port: port};

    console.log('added peer', peer);
    peers.push(peer);
}

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
    var peerId = join(ip, port);

    if(!peerId) {
        console.error('bootstrap failed, peer offline?');
        return true;
    }

    var peerRoutingTable = copyRoutingTable(peerId);
    if(!routingTable) {
        console.error('failed to copy peer routing table');
        return true;   
    }

    buildRoutingTable(peerRoutingTable);

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
        addPeer(rinfo.address, rinfo.port);
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
    console.log(`(give your bootstrap info to people to connect to you)`);

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
            holePunch(results);
            return;
        }
        
        getIP();
    });
}

function listMappings() {
    upnpClient.getMappings(function(err, results) {
        killError(err);
        for(ea in results) {
            var description = results[ea]['description'];
            var privateIP = results[ea]['private']['host'];
            var udp = results[ea]['protocol'] === 'udp';

            if(udp && description == 'Party line!') {
                console.log(results[ea]);
            } 
        } 
    });
}

// init();
listMappings();
