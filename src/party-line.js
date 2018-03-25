var ip = require('ip');
var dgram = require('dgram');
var crypto = require('crypto');
var natpmp = require('nat-pmp');
var keypair = require('keypair');
var natupnp = require('nat-upnp');

var Net = require('./network.js');
var Utils = require('./utils.js');
var Interface = require('./interface.js');

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
    ui.logMsg('bootstrapping to', globalConfig['bootstrapPeer']);

    net.join(pair, ip, port, peerId);

    return true;
}

function dumpPeerTable() {
    ui.logMsg(globalConfig['peerTable'][0]);
    ui.logMsg(globalConfig['peerTable'][128]);
    ui.logMsg(globalConfig['peerTable'][255]);
    return true;
}

function dumpKeyTable() {
    ui.logMsg(globalConfig['keyTable']);
    return true;
}

// create named chat

// join chat
// exchange public keys 
// do dh key exchange
// distribute chat id and shared secret
// distribute participant list

// new peer announces / exchanges keys with participants

function showHelp() {
    ui.logMsg('==== HELPFUL ====');
    ui.logMsg('');
    ui.logMsg('==== BOOTSTRAP');
    ui.logMsg('/bootstrap <peer\'s bootstrap info>');
    ui.logMsg('bootstrap your initial connection to a peer');
    ui.logMsg('aka: /bs');
    ui.logMsg('');
    ui.logMsg('==== LEAVE');
    ui.logMsg('/leave');
    ui.logMsg('notify peers of departure and close ports');
    ui.logMsg('aka: /exit /quit');
    ui.logMsg('');
    ui.logMsg('==== ENDHELP ====');
    return true;
}

function showId() {
    ui.logMsg(globalConfig['id']);
    return true;
}

function openSecure() {
    ui.logMsg(globalConfig['id']);
    return true;
}

// TODO /open peerid
function openSecure(toks) {
    if(toks.length < 2) {
        return false;
    }

    var targetId = toks[1];
    if(!targetId.match(/^[a-zA-Z0-9]{64}$/)) {
        return false;
    }

    net.setupSecure(globalConfig['pair'], targetId);
    return true;
}

// TODO /msg peerid words words words
function sendSecure(toks) {
    if(toks.length < 3) {
        return false;
    }

    var targetId = toks[1];
    if(!targetId.match(/^[a-zA-Z0-9]{64}$/)) {
        return false;
    }

    if(!(targetId in globalConfig['secretTable'])) {
        return false;
    }

    var msg = toks.slice(2).join(' ');
    net.sendPrivateMessage(globalConfig['pair'], targetId, msg);
    return true;
}

function handleCommand(command) {
    var toks = command.split(' ');
    var cmd = toks[0];
    
    switch(cmd) {
        case '/id':
            return showId();
        case '/open':
            return openSecure(toks);
        case '/msg':
            return sendSecure(toks);
        case '/bootstrap':
        case '/bs':
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
            return net.leave(pair, unmapPorts);
        case '/help':
            return showHelp();
        default:
            break;
    }
    
    // kill attempts to message before initialized
    if(!globalConfig['verified'] || !globalConfig['routingTableBuilt'])
        return true;

    return false;
}

function serverInit() {
    ui.logMsg('setting up socket...');
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
            case 'connectivity_check':
                net.onConnectivityCheck(pair, msgJSON);
                break;
            case 'connectivity_confirm':
                net.onConnectivityConfirm(pair, msgJSON);
                break;
            case 'query_key':
                net.onQueryKey(pair, msgJSON);
                break;
            case 'response_key':
                net.onResponseKey(pair, msgJSON);
                break;
            case 'chat':
                net.onChat(msgJSON);
                break;
            case 'setup_secure':
                net.onSetupSecure(pair, msgJSON);
                break;
            case 'finalize_secure':
                net.onFinalizeSecure(pair, msgJSON);
                break;
            case 'private_message':
                net.onPrivateMessage(pair, msgJSON);
                break;
            case 'private_message_receipt':
                net.onPrivateMessageReceipt(pair, msgJSON);
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

    ui.logMsg(`bootstrap info: ${bootstrapInfo}`);
    globalConfig['bootstrapInfo'] = bootstrapInfo;

    ui.logMsg(`bootstrap to a peer or have a peer bootstrap to you to get started`);
    setInterval(net.connectivityCheck, 10000);
}

function killError(err) {
    if(err) {
        ui.stop();
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
        ui.logMsg(ip);
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

    ui.logMsg('chose port', port);

    upnpClient.portMapping({
        public: globalConfig['ext']['port'],
        private: globalConfig['int']['port'],
        ttl: 0,
        protocol: 'UDP',
        description: 'Party line!',
        local: false,
    }, function(err) {
        killError(err);
        ui.logMsg('mapped port...');
        init();
    });
}

function init() {
    if(ip.address().match(/(^127\.)|(^192\.168\.)|(^10\.)|(^172\.1[6-9]\.)|(^172\.2[0-9]\.)|(^172\.3[0-1]\.)|(^::1$)|(^[fF][cCdD])/)) {
        ui.logMsg('attempting upnp...')
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
                    ui.logMsg('found already open port');
                    globalConfig['int']['port'] = results[ea]['private']['port'];
                    globalConfig['ext']['port'] = results[ea]['public']['port'];
                    break;
                } 
            } 

            if(!found) {
                ui.logMsg('open port not found, mapping');
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

function unmapPorts() {
    if(ip.address().match(/(^127\.)|(^192\.168\.)|(^10\.)|(^172\.1[6-9]\.)|(^172\.2[0-9]\.)|(^172\.3[0-1]\.)|(^::1$)|(^[fF][cCdD])/)) {
        ui.logMsg('unmapping ports...');
        upnpClient.getMappings(function(err, results) {
            killError(err);
            for(ea in results) {
                var description = results[ea]['description'];
                var privateIp = results[ea]['private']['host'];
                var udp = results[ea]['protocol'] === 'udp';
                var port = results[ea]['public']['port'];

                if(udp && description == 'Party line!' && privateIp == globalConfig['int']['ip']) {
                    ui.logMsg(`unmapping ${port}...`);
                    if(!('unmap' in globalConfig)) {
                        globalConfig['unmap'] = 0;
                    }
                    globalConfig['unmap'] += 1;
                    upnpClient.portUnmapping(
                        {public: port, protocol: 'UDP'}, 
                        function(port, err, res) {
                            globalConfig['unmap'] -= 1;
                            
                            if(err) {
                                ui.logMsg(`err unmapping: ${err}`);
                            } else {
                                ui.logMsg(`unmapped ${port}`);
                            }
                            
                            if(globalConfig['unmap'] == 0) {
                                ui.logMsg('safe to exit (F4) now...')
                            }
                        }.bind(undefined, port));
                } 
            } 
        });
    } else {
        ui.logMsg('safe to exit (F4) now...')
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
                ui.logMsg(results[ea]);
            } 
        } 
    });
}

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

var net = new Net(globalConfig, handleCommand);
var utils = net.utils;
var ui = net.ui;
ui.start();

// var stdin = process.openStdin();
globalConfig['peerTable'] = new Array(256);
globalConfig['keyTable'] = {};
globalConfig['secretTable'] = {};

ui.logMsg('generating keypair...');
var pair = keypair({bits: 2048}); 
var id = utils.sha256(pair.public);
ui.logMsg(`id: ${id}`);

ui.logMsg('generating ephemeral keys...');
var dh = crypto.createECDH('secp521r1');
var dhPub = dh.generateKeys('hex');
ui.logMsg(`initializing server...`);

globalConfig['id'] = id;
globalConfig['pair'] = pair;
globalConfig['dh'] = dh;

globalConfig['idealRoutingTable'] = utils.calculateIdealRoutingTable(globalConfig['id']);
globalConfig['peerCandidates'] = [];
globalConfig['chatMessages'] = new Array(512);
globalConfig['chatMessagesReceived'] = {};

// TODO: create image with fingerprint

// ui.logMsg('generating keys...');
// var dh2 = crypto.createDiffieHellman(dh.getPrime(), dh.getGenerator());
// var pubKey2 = dh2.generateKeys('hex');
// ui.logMsg(`pubkey ${pubKey2}`);
// // id is sha512 of public key
// var fingerprint2 = crypto.createHash('sha256').update(Buffer.from(pubKey2, 'hex')).digest('hex');
// ui.logMsg(`fingerprint: ${fingerprint2}`);

// dh2.computeSecret(pubKey, 'hex');
// var sessionSecret = dh.computeSecret(pubKey2, 'hex');

init();
// listMappings();
