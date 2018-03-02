var natpmp = require('nat-pmp');
var natupnp = require('nat-upnp');
var async = require('async');
var dgram = require('dgram');
var ip = require('ip');
var crypto = require('crypto');
var keypair = require('keypair');

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

function sha256(data) {
    return crypto.createHash('sha256').update(data).digest('hex');
}

function sign(data) {
    var signer = crypto.createSign('SHA256');
    signer.update(data);

    var privkey = globalConfig['pair'].private;
    var sig = signer.sign(privkey, 'hex');
    return sig;
}

function verify(pubkey, sig, data) {
    var verifier = crypto.createVerify('sha256');
    verifier.update(data);
    return verifier.verify(pubkey, sig, 'hex');
}

var stdin = process.openStdin();
// peer {ip: '6.6.6.6', port: 0x1337}
var peerTable = new Array(256);
var keyTable = {};

console.log('generating keypair...');
var pair = keypair({bits: 2048}); 
var id = sha256(pair.public);
console.log(`id: ${id}`);

console.log('generating ephemeral keys...');
var dh = crypto.createDiffieHellman(1024);
var pubkey = dh.generateKeys('hex');
console.log(`complete!`);

globalConfig['id'] = id;
globalConfig['pair'] = pair;
globalConfig['dh'] = dh;

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

function bufferXor(a, b) {
    var length = Math.max(a.length, b.length)
    var buffer = Buffer.allocUnsafe(length)

    for (var i = 0; i < length; ++i) {
         buffer[i] = a[i] ^ b[i];
    }

    return buffer
}

function calculateIdealRoutingTable() {
    var idealPeerList = [];
    for(var i = 0; i < 256; i++) {
        var powerBuf = Buffer.from((2**i).toString(16).padStart(64, '0'), 'hex');
        var idealPeer = bufferXor(globalConfig['id'], powerBuf);
        idealPeerList.push(idealPeer);
    }
    return idealPeerList;
}

var idealRoutingTable = calculateIdealRoutingTable();

function testSelfConnection() {
    // TODO
    // connect on ext ip, port to self
    // this should validate that the port is open
}

function send(peer, data) {
    // send message asking for closest
    var socket = globalConfig['socket'];
    // connect to peer
    var pubkey = globalConfig['pair'].public;
    var dataJSON = JSON.stringify(data);
    var sig = sign(dataJSON);

    var msg = {
        data: dataJSON,
        sig: sig,
    };

    var msgJSON = JSON.stringify(msg);
    console.log(`sending: ${msgJSON}`)
    socket.send(msgJSON, peer['port'], peer['ip']);
}

function flood(data) {
    var sent = [];
    for(var i = 0; i < 256; i++) {
        var peer = peerTable[i];
        if(send.indexOf(peer['id'] > -1)) {
            continue;
        }

        send(peer, data);
        sent.push(peer['id']);
    }
}

// join
function join(ip, port, id) {
    var socket = globalConfig['socket'];
    // connect to peer
    var pubkey = globalConfig['pair'].public;

    var data = {
        type: 'join',
        id: globalConfig['id'],
        ip: globalConfig['ext']['ip'],
        port: globalConfig['ext']['port'],
        key: pubkey,
    };

    send({ip, port, id}, data);

    // get peerId
    // get peer public key
    // add peer to routing table with key
    // return peerId
}

function onJoin(msgJSON) {
    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    if(!verify(data['pubkey'], sig, msg['data'])) {
        console.log('failed to verify join');
        return;
    }

    // send verify
    var socket = globalConfig['socket'];

    var pubkey = pair.public;
    var responseData = {
        type: 'verify',
        ip: globalConfig['ext']['ip'],
        port: globalConfig['ext']['port'],
        key: pubkey,
        verify: sha256(data['key']),
        peerTable: peerTable,
        keyTable: keyTable,
    };
    
    var peer = {
        port: data['port'],
        ip: data['ip'],
    }
    send(peer, responseData);
}

// build node routing table
function buildRoutingTable(seedTable, extra) {
    var fullTable = seedTable;
    
    if(extra) {
        fullTable = seedTable.concat(extra);
    }

    for(var i = 0; i < 256; i++) {
        var idealPeerId = idealRoutingTable[i];
        var closestPeer = findClosest(fullTable, idealPeerId);
        peerTable[i] = closestPeer;
    }

    announce();

    for(var i = 0; i < 256; i++) {
        var targetId = idealRoutingTable[i];
        var peer = peerTable[i];
        queryClosest(peer, targetId);
    }
}

function onVerify(msgJSON) {
    if(globalConfig['verified']) {
        return;
    }

    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    var peerIp = data['ip'];
    var peerPort = data['port'];
    var peerPubkey = data['pubkey'];

    var bootstrapIp = globalConfig['bootstrapPeer']['ip'];
    var bootstrapPort = globalConfig['bootstrapPeer']['port'];
    var bootstrapId = globalConfig['bootstrapPeer']['id'];

    var peerId = sha256(peerPubkey);
    var peerIdMatch = peerId === bootstrapId;
    var selfIdMatch = data['verify'] === globalConfig['id'];
    var dataIntegrity = verify(peerPubkey, sig, msg['data']);

    if (peerIdMatch && selfIdMatch && dataIntegrity) {
        globalConfig['verified'] = true;
    } else {
        console.error('ABORT: SIGNATURE VERIFICATION FAILED ON BOOTSTRAP PEER');
        process.exit(1);
    }

    globalConfig['bootstrapPeer']['ip'] = peerIp;
    globalConfig['bootstrapPeer']['port'] = peerPort;
    globalConfig['bootstrapPeer']['pubkey'] = peerPubkey;

    var peerTable = data['peerTable'];

    keyTable = data['keyTable'];
    for(var keyId in keyTable) {
        if(keyId != sha256(keyTable[keyId])) {
            delete keyTable[keyId];
        }
    }
    keyTable[peerId] = peerPubkey;

    buildRoutingTable(peerTable, globalConfig['bootstrapPeer']);
    globalConfig['routingTableBuilt'] = true;
}

function announce() {
    // announce self
    var pubkey = pair.public;
    var data = {
        type: 'announce',
        id: globalConfig['id'],
        ip: globalConfig['ext']['ip'],
        port: globalConfig['ext']['port'],
        key: pubkey,
    };

    flood(data);
}

function floodReplay(data) {
    var sent = [];
    for(var i = 0; i < 256; i++) {
        var peer = peerTable[i];
        if(peer['id'] in sent) {
            continue;
        }

        socket.send(data, peer['port'], peer['ip']);
        sent.push(peer['id']);
    }
}

function onAnnounce() {
    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    // check have key
    var peerPubkey = data['key'];
    var peerId = data['id'];
    if(peerId in keyTable) {
        return;
    }

    // verify key
    var pubkeyHash = sha256(peerPubkey);
    var peerIdMatch = peerId === pubkeyHash;
    var dataIntegrity = verify(peerPubkey, sig, msg['data']);

    if(!(peerIdMatch && dataIntegrity)) {
        return;
    }

    // check update routing table
    keyTable[peerId] = peerPubkey;
    var peer = {
        id: peerId,
        ip: data['ip'],
        port: data['port'],
        key: peerPubkey,
    }

    var idealMatches = wouldUpdateTable(peer);
    for(var idx in idealMatches) {
        var idealPeerId = idealMatches[idx];
        queryClosest(peer, idealPeerId);
    }

    // propagate
    floodReplay(msgJSON);
}

function wouldUpdateTable(peer) {
    var idealMatches = [];
    for(var i = 0; i < 256; i++) {
        var targetId = idealRoutingTable[i];
        var currPeer = peerTable[i];
        var targetIdBuf = Buffer.from(targetId, 'hex');
        var currIdBuf = Buffer.from(currPeer['id'], 'hex');
        var peerIdBuf = Buffer.from(peer['id'], 'hex');

        var currDistance = bufferXor(targetIdBuf, currIdBuf);
        var peerDistance = bufferXor(targetIdBuf, peerIdBuf);

        if(peerDistance.compare(currDistance) < 0) {
            idealMatches.push(targetId);
        }
    }
    return idealMatches;
}

function updateTable(peer) {
    var idealMatches = [];
    for(var i = 0; i < 256; i++) {
        var targetId = idealRoutingTable[i];
        var currPeer = peerTable[i];
        var targetIdBuf = Buffer.from(targetId, 'hex');
        var currIdBuf = Buffer.from(currPeer['id'], 'hex');
        var peerIdBuf = Buffer.from(peer['id'], 'hex');

        var currDistance = bufferXor(targetIdBuf, currIdBuf);
        var peerDistance = bufferXor(targetIdBuf, peerIdBuf);

        if(peerDistance.compare(currDistance) < 0) {
            idealMatches.push(targetId);
            peerTable[i] = peer;
        }
    }

    return idealMatches;
}

// leave
function leave() {
    var closest = findClosest(peerTable, globalConfig['id']);
    var pubkey = pair.public;
    var data = {
        type: 'leave',
        id: globalConfig['id'],
        closest: closest,
    };

    flood(data);
}

function findClosestExclude(searchTable, targetId, excludeId) {
    var targetIdBuf = Buffer.from(targetId, 'hex');

    var closest;
    for(var idx in searchTable) {
        if(searchTable[idx]['id'] == excludeId) {
            continue;
        }
        closest = searchTable[idx];
        break;
    }

    if(closest === undefined) {
        return undefined;
    }

    var closestIdBuf = Buffer.from(closest['id'], 'hex');
    var closestDist = bufferXor(closestIdBuf, targetIdBuf);

    for(var idx in searchTable) {
        var peer = searchTable[idx];
        var peerId = peer['id'];

        if(peerId === excludeId) {
            continue;
        }

        var peerIdBuf = Buffer.from(peerId, 'hex');
        var peerDist = bufferXor(peerIdBuf, targetIdBuf);
        if(peerDist.compare(closestDist) < 0) {
            closest = peer;
            closestDist = peerDist;
        }
    }

    return closest;
}

function updateTableRemove(peerId) {
    var contains = false;
    var indices = [];
    for(var i = 0; i < 256; i++) {
        if(peerTable[i]['id'] === peerId) {
            indices.push(i);
        }
    }

    if(indices.length === 0) {
        return;
    }

    var idealIds = [];
    for(var j in indices) {
        var idx = indices[j];
        var peer = peerTable[idx];
        var idealPeerId = idealRoutingTable[idx];
        idealIds.push(idealPeerId);
        var closest = findClosestExclude(peerTable, idealPeerId, peer['id']);
        
        if(closest === undefined) {
            delete peerTable[idx];
            continue;
        }

        peerTable[idx] = closest;
    }
    return idealIds;
}

function onLeave() {
    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    // check have key
    var peerId = data['id'];
    if(!(peerId in keyTable)) {
        return;
    }

    var peerPubkey = keyTable[peerId];

    // verify key
    var pubkeyHash = sha256(peerPubkey);
    var peerIdMatch = peerId === pubkeyHash; // I think this is unecessary
    var dataIntegrity = verify(peerPubkey, sig, msg['data']);

    if(!(peerIdMatch && dataIntegrity)) {
        return;
    }

    // check update routing table
    delete keyTable[peerId];
    var idealIds = updateTableRemove(peerId);
    for(var idx in idealIds) {
        var idealPeerId = idealIds[idx];
        queryClosest(data['closest'], idealPeerId);
    }

    // propagate
    floodReplay(msgJSON);
}
// Buffer.compare(b1, b2)
// b1.compare(b2)
// -1 first is less
// 1 second is less

function findClosest(searchTable, targetId) {
    var targetIdBuf = Buffer.from(targetId, 'hex');

    var closest = searchTable[0];
    var closestIdBuf = Buffer.from(closest['id'], 'hex');
    var closestDist = bufferXor(closestIdBuf, targetIdBuf);

    for(var idx in searchTable) {
        var peer = searchTable[idx];
        var peerId = peer['id'];
        var peerIdBuf = Buffer.from(peerId, 'hex');
        var peerDist = bufferXor(peerIdBuf, targetIdBuf);
        if(peerDist.compare(closestDist) < 0) {
            closest = peer;
            closestDist = peerDist;
        }
    }

    return closest;
}

var peerCandidates = [];

// find closest
function queryClosest(peer, targetId) {
    // send message asking for closest
    var data = {
        type: 'query_closest',
        id: globalConfig['id'],
        ip: globalConfig['ext']['ip'],
        port: globalConfig['ext']['port'],
        target: targetId,
    };
    peerCandidates.push(peer['id']);
    send(peer, data);
}

function onQueryClosest(msgJSON) {
    // update peerTable
    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    var peerId = data['id'];
    var peerPubkey = keyTable[peerId];

    if(!peerPubkey || !verify(peerPubkey, sig, msg['data'])) {
        return;
    }

    var closest = findClosest(peerTable, data['target'], true);
    var pubkey = pair.public;
    var peerSelf = {
        id: globalConfig['id'],
        ip: globalConfig['ext']['ip'],
        port: globalConfig['ext']['port'],
        key: pubkey,
    };

    var targetIdBuf = Buffer.from(data['target'], 'hex');
    var closestDist = bufferXor(Buffer.from(closest['id'], 'hex'), targetIdBuf);
    var selfDist = bufferXor(Buffer.from(peerSelf['id'], 'hex'), targetIdBuf);
    
    var responseData = {
        type: 'response_closest',
        closest: closest,
        from: peerSelf,
        self: false,
    };

    if(selfDist.compare(closestDist) < 0) {
        responseData['closest'] = peerSelf;
        responseData['self'] = true;
    }

    var peer = {port: data['port'], ip: data['ip']};
    send(peer, responseData);
}

function onResponseClosest(msgJSON) {
    // update peerTable
    var msg = JSON.parse(msgJSON);
    var data = JSON.parse(msg['data']);
    var sig = msg['sig'];

    if(!(data['from']['id'] in keyTable) || peerCandidates.indexOf(data['from']['id']) < 0) {
        return;
    }

    var peerPubkey = keyTable[data['from']['id']];
    var dataIntegrity = verify(peerPubkey, sig, msg['data']);
    var keyMatch = data['from']['pubkey'] === peerPubkey;

    if(!dataIntegrity || !keyMatch) {
        return;
    }

    // update routing table with peer who replied
    var peer = data['from'];
    updateTable(peer);
    peerCandidates = peerCandidates.filter(function(peerId) {
        return peerId != peer['id'];
    });

    // ask suggested closest for ideal closest
    if(!data['self']) {
        var idealMatches = wouldUpdateTable(closest);
        for(var idx in idealMatches) {
            var idealPeerId = idealMatches[idx];
            queryClosest(closest, idealPeerId);
        }
    }
}

function bootstrap(toks) {
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
    var id = match[3];

    var ip = [
        parseInt(ipEnc.substr(0, 2), 16),
        parseInt(ipEnc.substr(2, 2), 16),
        parseInt(ipEnc.substr(4, 2), 16),
        parseInt(ipEnc.substr(6, 2), 16),
    ].map(function(ea) {
        return ea.toString();
    }).join('.');

    var port = parseInt(portEnc, 16);
    
    globalConfig['bootstrapPeer'] = {ip, port, id};

    join(ip, port, id);

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

    socket.on('message', (msgJSON, rinfo) => {
        console.log(`server got: ${msgJSON} from ${rinfo.address}:${rinfo.port}`);
        // handle external message
        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        switch(data['type']) {
            case 'join':
                onJoin(msgJSON);
                break;
            case 'verify':
                onVerify(msgJSON);
                break;
            case 'leave':
                onLeave(msgJSON);
                break;
            case 'announce':
                onAnnounce(msgJSON);
                break;
            case 'query_closest':
                onQueryClosest(msgJSON);
                break;
            case 'response_closest':
                onResponseClosest(msgJSON);
                break;
            case 'chat':
                onChat(msgJSON);
                break;
            default:
                break;
        }
    });

    socket.on('listening', () => {
        const address = socket.address();
        console.log(`server listening ${address.address}:${address.port}`);
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
    console.log(`(give your bootstrap info to people to connect to you)`);

    stdin.addListener("data", function(d) {
        // note:  d is an object, and when converted to a string it will
        // end with a linefeed.  so we (rather crudely) account for that  
        // with toString() and then trim() 
        var input = d.toString().trim();
        console.log(`you entered: [${input}]`);

        if(!handleCommand(input)) {
            // handle userInput
            for(idx in peerTable) {
                var peer = peerTable[idx];
                var data = {
                    type: 'chat',
                    msg: input,
                }
                send(peer, data);
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
    if(ip.address().match(/(^127\.)|(^192\.168\.)|(^10\.)|(^172\.1[6-9]\.)|(^172\.2[0-9]\.)|(^172\.3[0-1]\.)|(^::1$)|(^[fF][cCdD])/)) {
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
            
            getIp();
        });
    } else {
        globalConfig['int']['port'] = 0xdab;
        globalConfig['ext']['port'] = 0xdab;
        globalConfig['int']['ip'] = ip.address();
        globalConfig['ext']['ip'] = ip.address();
        serverInit();
    }
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
