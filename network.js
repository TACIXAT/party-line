var Utils = require('./utils.js');
var Interface = require('./interface.js');

module.exports = function(globalConfig, handleCommand) {
    var ui = new Interface(globalConfig);
    ui.setEnterCallback(function(ch, key) {
        var msg = this.value.trim();
        if(!handleCommand(msg)) {
            // handle userInput
            var data = {
                type: 'chat',
                id: globalConfig['id'],
                ts: Date.now(),
                content: msg,
            }
            // utils.addChat(data);
            module.flood(globalConfig['pair'], data);
        }
        this.clearValue();
    });
    module.ui = ui;

    var utils = new Utils(globalConfig, module, ui);
    module.utils = utils;

    module.testSelfConnection = function() {
        // TODO
        // connect on ext ip, port to self
        // this should validate that the port is open
    }

    module.send = function(peer, pair, data) {
        // send message asking for closest
        var socket = globalConfig['socket'];
        // connect to peer
        var pubkey = pair.public;
        var dataJSON = JSON.stringify(data);
        var sig = utils.sign(pair, dataJSON);

        var msg = {
            data: dataJSON,
            sig: sig,
        };

        var msgJSON = JSON.stringify(msg);
        socket.send(msgJSON, peer['port'], peer['ip']);
    }

    module.flood = function(pair, data) {
        var sent = [];
        for(var i = 0; i < 256; i++) {
            var peer = globalConfig['peerTable'][i];
            if(!peer || sent.indexOf(peer['id']) > -1) {
                continue;
            }

            module.send(peer, pair,  data);
            sent.push(peer['id']);
        }
    }

    // join
    module.join = function(pair, ip, port, peerId) {
        ui.logMsg('sending join...');
        var socket = globalConfig['socket'];
        // connect to peer
        var pubkey = pair.public;

        var data = {
            type: 'join',
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            key: pubkey,
            bsId: peerId,
        };

        module.send({ip, port, peerId}, pair, data);

        // get peerId
        // get peer public key
        // add peer to routing table with key
        // return peerId
    }

    module.onJoin = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['key', 'bsId', 'port', 'ip'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        if(!utils.verify(data['key'], sig, msg['data'])) {
            ui.logMsg('verify failed');
            return;
        }

        if(data['bsId'] !== globalConfig['id']) {
            ui.logMsg('bsId no match', bsId, globalConfig['id']);
            return;
        }

        // send verify
        var pubkey = pair.public;
        var responseData = {
            type: 'verify',
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            key: pubkey,
            verify: utils.sha256(data['key']),
        };
        
        var peer = {
            port: data['port'],
            ip: data['ip'],
        }
 
        ui.logMsg('sending verify...');
        module.send(peer, pair, responseData);
        globalConfig['verified'] = true;
    }

    module.onVerify = function(pair, msgJSON) {
        if(globalConfig['verified']) {
            return;
        }

        if(!utils.validateMsg(msgJSON, ['ip', 'port', 'key', 'verify'])) {
            return;
        }

        ui.logMsg('received verify...');

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerIp = data['ip'];
        var peerPort = data['port'];
        var peerPubkey = data['key'];

        // compare hash of peer pubkey to known bootstrap id
        var peerId = utils.sha256(peerPubkey);
        var peerIdMatch = peerId === globalConfig['bootstrapPeer']['id'];
        var selfIdMatch = data['verify'] === globalConfig['id'];
        // validate signature
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);

        if (peerIdMatch && selfIdMatch && dataIntegrity) {
            globalConfig['verified'] = true;
        } else {
            console.error('ABORT: SIGNATURE VERIFICATION FAILED ON BOOTSTRAP PEER');
            process.exit(1);
        }

        // store peer's info
        globalConfig['bootstrapPeer']['ip'] = peerIp;
        globalConfig['bootstrapPeer']['port'] = peerPort;
        globalConfig['bootstrapPeer']['key'] = peerPubkey;

        globalConfig['keyTable'][peerId] = peerPubkey;

        // sets bootstrap peer as every entry in peer table
        utils.buildRoutingTable(pair);

        ui.logMsg('announcing to network...');
        module.announce(pair);

        // TODO: test this without delay
        ui.logMsg('querying peers for closest...');
        var peer = globalConfig['bootstrapPeer'];
        for(var i = 0; i < 256; i++) {
            var targetId = globalConfig['idealRoutingTable'][i];
            setTimeout(module.queryClosest.bind(undefined, pair, peer, targetId), 4 * (i+1));
        }

        setTimeout(function() {
            ui.logMsg('happy chatting!');
        }, 4 * 258);

        globalConfig['routingTableBuilt'] = true;

    }

    module.announce = function(pair) {
        // announce self
        var pubkey = pair.public;
        var data = {
            type: 'announce',
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            key: pubkey,
        };

        module.flood(pair, data);
    }

    module.floodReplay = function(data) {
        var socket = globalConfig['socket'];
        var sent = [];
        for(var i = 0; i < 256; i++) {
            var peer = globalConfig['peerTable'][i];
            if(!peer || sent.indexOf(peer['id']) > -1) {
                continue;
            }

            socket.send(data, peer['port'], peer['ip']);
            sent.push(peer['id']);
        }
    }

    module.onAnnounce = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['key', 'id', 'ip', 'port'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        // check have key
        var peerPubkey = data['key'];
        var peerId = data['id'];
        if(peerId in globalConfig['keyTable'] || peerId == globalConfig['id']) {
            return;
        }

        // verify key
        var pubkeyHash = utils.sha256(peerPubkey);
        var peerIdMatch = peerId === pubkeyHash;
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);

        if(!(peerIdMatch && dataIntegrity)) {
            return;
        }

        // check update routing table
        globalConfig['keyTable'][peerId] = peerPubkey;
        var peer = {
            id: peerId,
            ip: data['ip'],
            port: data['port'],
            key: peerPubkey,
        }

        var idealMatches = utils.wouldUpdateTable(peer);
        ui.logMsg(`announce got, querying peer ${idealMatches.length} times`);
        for(var idx in idealMatches) {
            var idealPeerId = idealMatches[idx];
            setTimeout(module.queryClosest.bind(undefined, pair, peer, idealPeerId), 10 * (idx+1));
        }
        
        // propagate
        module.floodReplay(msgJSON);
    }

    // find closest
    module.queryClosest = function(pair, peer, targetId) {
        // send message asking for closest
        var data = {
            type: 'query_closest',
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            target: targetId,
        };

        // store the peer id to filter responses
        // this prevents users from "suggesting" non-existent peers
        globalConfig['peerCandidates'].push(peer['id']);
        if(utils.sha256(peer['key']) !== peer['id']) {
            return;
        }

        globalConfig['keyTable'][peer['id']] = peer['key'];
        module.send(peer, pair, data);
    }

    module.onQueryClosest = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['id', 'target', 'port', 'ip'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerId = data['id'];
        // var peerPubkey = globalConfig['keyTable'][peerId];

        // if(!peerPubkey || !utils.verify(peerPubkey, sig, msg['data'])) {
        //     return;
        // }

        // find the closest to the requested id, exclude requesting peer
        var closest = utils.findClosestExclude(globalConfig['peerTable'], data['target'], [data['id']]);
        var pubkey = pair.public;
        var peerSelf = {
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            key: pubkey,
        };

        // calculate distances between requested and self
        var targetIdBuf = Buffer.from(data['target'], 'hex');
        var selfDist = utils.bufferXor(Buffer.from(peerSelf['id'], 'hex'), targetIdBuf);
        var closestDist = selfDist;

        // calculate distance between requested and found
        if(closest) {
            closestDist = utils.bufferXor(Buffer.from(closest['id'], 'hex'), targetIdBuf);
        } 
        
        // set closest
        var responseData = {
            type: 'response_closest',
            closest: closest,
            from: peerSelf,
            self: false,
        };

        // set self if closer or closest dne
        if(!closest || selfDist.compare(closestDist) < 0) {
            responseData['closest'] = peerSelf;
            responseData['self'] = true;
        }

        var peer = {port: data['port'], ip: data['ip']};
        module.send(peer, pair, responseData);
    }

    module.onResponseClosest = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['from', 'closest', 'self'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        // user should be in keytable and peercandidates
        if(!(data['from']['id'] in globalConfig['keyTable']) || globalConfig['peerCandidates'].indexOf(data['from']['id']) < 0) {
            return;
        }

        // get key by id and check message signature
        var peerPubkey = globalConfig['keyTable'][data['from']['id']];
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);
        var keyMatch = data['from']['key'] === peerPubkey;

        if(!dataIntegrity || !keyMatch) {
            return;
        }

        // update routing table with peer who replied
        var peer = data['from'];
        var added = utils.updateTable(peer);
        globalConfig['peerCandidates'] = globalConfig['peerCandidates'].filter(function(peerId) {
            return peerId != peer['id'];
        });

        // mark routing table as built if it wasn't
        if(!globalConfig['routingTableBuilt']) {
            globalConfig['routingTableBuilt'] = true;
            ui.logMsg('peer table built...');
            ui.logMsg('happy chatting!');
        }

        // ask suggested closest for ideal closest
        var closest = data['closest'];
        if(!data['self']) {
            var idealMatches = utils.wouldUpdateTable(closest);
            for(var idx in idealMatches) {
                var idealPeerId = idealMatches[idx];
                module.queryClosest(pair, closest, idealPeerId);
            }
        }
    }

    // leave
    module.leave = function(pair) {
        // prevent replies to anyone
        module.onJoin = function() { return; };
        module.onVerify = function() { return; };
        module.onLeave = function() { return; };
        module.onAnnounce = function() { return; };
        module.onQueryClosest = function() { return; };
        module.onResponseClosest = function() { return; };
        module.onChat = function() { return; };

        // TODO: don't fail to send if closest not found
        var sent = [];
        for(var i = 0; i < 256; i++) {
            var peer = globalConfig['peerTable'][i];
            if(!peer || sent.indexOf(peer['id']) > -1) {
                continue;
            }

            var closest = utils.findClosestExclude(globalConfig['peerTable'], globalConfig['id'], [globalConfig['id'], peer['id']]);
            var pubkey = pair.public;
            var data = {
                type: 'leave',
                id: globalConfig['id'],
                closest: closest,
            };

            module.send(peer, pair, data);
            sent.push(peer['id']);
        }

        ui.logMsg('safe to exit (ctrl + c) now...');
        return true;
    }

    module.onLeave = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['id'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        // check have key
        var peerId = data['id'];
        if(!(peerId in globalConfig['keyTable'])) {
            return;
        }

        var peerPubkey = globalConfig['keyTable'][peerId];

        // verify key
        var pubkeyHash = utils.sha256(peerPubkey);
        var peerIdMatch = peerId === pubkeyHash; // I think this is unecessary
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);

        if(!(peerIdMatch && dataIntegrity)) {
            return;
        }

        // check update routing table
        delete globalConfig['keyTable'][peerId];
        var idealIds = utils.updateTableRemove(peerId);

        var tableEmpty = globalConfig['peerTable'].filter(function(ea) { return ea }).length == 0;
        if(tableEmpty) {
            globalConfig['verified'] = false;
            globalConfig['routingTableBuilt'] = false;
            
            if(!data['closest']) {
                // we're alone, don't flood message or query for closest to yourself
                ui.logMsg('no peers remaining :(');
                ui.logMsg('bootstrap some new ones!');
                ui.logMsg(globalConfig['bootstrapInfo']);
                return;
            }
        }

        if(data['closest']) {
            // query suggested closest to replace peer that left
            for(var idx in idealIds) {
                var idealPeerId = idealIds[idx];
                module.queryClosest(pair, data['closest'], idealPeerId);
            }
        }

        // propagate
        module.floodReplay(msgJSON);
    }

    module.queryKey = function(targetId) {
        var data = {
            type: 'query_key',
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            target: targetId,
        };

        // get closest peer
        var closestPeer = utils.findClosest(globalConfig['peerTable'], targetId);

        if(!closestPeer) {
            return;
        }

        module.send(closestPeer, globalConfig['pair'], data);
    }

    module.onQueryKey = function(msgJSON) {
        if(!utils.validateMsg(msgJSON, ['id', 'ip', 'port', 'target'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        if(data['id'] === globalConfig['id']) {
            // get key
            var data = {
                type: 'reponse_key',
                key: globalConfig['pair'].public,
            }
        } else {
            var closestPeer = utils.findClosest(globalConfig['peerTable'], targetId);

            if(!closestPeer) {
                return;
            }            

            var data = {
                type: 'response_key', 
                closest: closestPeer,

            }

            module.send(data[''])
        }
    }

    module.onResponseKey = function(msgJSON) {

    }

    module.onChat = function(msgJSON) {
        if(!utils.validateMsg(msgJSON, ['content', 'id', 'ts'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerId = data['id'];
        if(!(peerId in globalConfig['keyTable'])) {
            data['verified'] = false;
            // TODO: query for user key
        } else if(!utils.verify(globalConfig['keyTable'][peerId], sig, msg['data'])) {
            return;
        }

        if(data['content'] == '' || utils.checkReceivedChat(data)) {
            return;
        }

        utils.addChat(data);
        module.floodReplay(msgJSON);
    }

    return module;
}