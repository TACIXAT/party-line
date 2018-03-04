var Utils = require('./utils.js');

module.exports = function(globalConfig) {
    var utils = new Utils(globalConfig, module);

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
        console.log('sending join...');
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
            console.log('verify failed');
            return;
        }

        if(data['bsId'] !== globalConfig['id']) {
            console.log('bsId no match', bsId, globalConfig['id']);
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
 
        console.log('sending verify...');
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

        console.log('received verify...');

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerIp = data['ip'];
        var peerPort = data['port'];
        var peerPubkey = data['key'];

        var bootstrapIp = globalConfig['bootstrapPeer']['ip'];
        var bootstrapPort = globalConfig['bootstrapPeer']['port'];
        var bootstrapId = globalConfig['bootstrapPeer']['id'];

        var peerId = utils.sha256(peerPubkey);
        var peerIdMatch = peerId === bootstrapId;
        var selfIdMatch = data['verify'] === globalConfig['id'];
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);

        if (peerIdMatch && selfIdMatch && dataIntegrity) {
            globalConfig['verified'] = true;
        } else {
            console.error('ABORT: SIGNATURE VERIFICATION FAILED ON BOOTSTRAP PEER');
            process.exit(1);
        }

        globalConfig['bootstrapPeer']['ip'] = peerIp;
        globalConfig['bootstrapPeer']['port'] = peerPort;
        globalConfig['bootstrapPeer']['key'] = peerPubkey;

        globalConfig['keyTable'][peerId] = peerPubkey;

        utils.buildRoutingTable(pair);
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
        if(peerId in globalConfig['keyTable']) {
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
        for(var idx in idealMatches) {
            var idealPeerId = idealMatches[idx];
            setTimeout(utils.delayQueryClosest.bind(undefined, pair, peer, idealPeerId), 20 * (idx+1));
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

        // update peerTable
        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerId = data['id'];
        var peerPubkey = globalConfig['keyTable'][peerId];

        if(!peerPubkey || !utils.verify(peerPubkey, sig, msg['data'])) {
            return;
        }

        var closest = utils.findClosestExclude(globalConfig['peerTable'], data['target'], [data['id']]);
        var pubkey = pair.public;
        var peerSelf = {
            id: globalConfig['id'],
            ip: globalConfig['ext']['ip'],
            port: globalConfig['ext']['port'],
            key: pubkey,
        };

        var targetIdBuf = Buffer.from(data['target'], 'hex');
        var selfDist = utils.bufferXor(Buffer.from(peerSelf['id'], 'hex'), targetIdBuf);
        var closestDist = selfDist;
        if(closest) {
            closestDist = utils.bufferXor(Buffer.from(closest['id'], 'hex'), targetIdBuf);
        } 
        
        var responseData = {
            type: 'response_closest',
            closest: closest,
            from: peerSelf,
            self: false,
        };

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

        // update peerTable
        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        if(!(data['from']['id'] in globalConfig['keyTable']) || globalConfig['peerCandidates'].indexOf(data['from']['id']) < 0) {
            return;
        }

        var peerPubkey = globalConfig['keyTable'][data['from']['id']];
        var dataIntegrity = utils.verify(peerPubkey, sig, msg['data']);
        var keyMatch = data['from']['key'] === peerPubkey;

        if(!dataIntegrity || !keyMatch) {
            return;
        }

        if(!globalConfig['routingTableBuilt']) {
            globalConfig['routingTableBuilt'] = true;
            console.log('peer table built...');
            console.log('happy chatting!');
        }

        // update routing table with peer who replied
        var peer = data['from'];
        utils.updateTable(peer);
        globalConfig['peerCandidates'] = globalConfig['peerCandidates'].filter(function(peerId) {
            return peerId != peer['id'];
        });

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
        console.log('leaving...')
        var closest = utils.findClosest(globalConfig['peerTable'], globalConfig['id']);
        if(closest) {
            var pubkey = pair.public;
            var data = {
                type: 'leave',
                id: globalConfig['id'],
                closest: closest,
            };

            module.flood(pair, data);
        }
        console.log('safe to exit (ctrl + c) now...');
        return true;
    }

    module.onLeave = function(pair, msgJSON) {
        if(!utils.validateMsg(msgJSON, ['id', 'closest'])) {
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
        
        // closest is self and empty peer table :(
        var tableEmpty = globalConfig['peerTable'].filter(function(ea) { return ea != null }).length == 0;
        if(data['closest']['id'] == globalConfig['id'] && tableEmpty) {
            globalConfig['verified'] = false;
            globalConfig['routingTableBuilt'] = false;
            // we're alone, don't flood message or query for closest to yourself
            console.log('no peers remaining :(');
            console.log('bootstrap some new ones!');
            console.log(globalConfig['bootstrapInfo']);
            return;
        }

        for(var idx in idealIds) {
            var idealPeerId = idealIds[idx];
            module.queryClosest(pair, data['closest'], idealPeerId);
        }

        // propagate
        module.floodReplay(msgJSON);
    }

    module.onChat = function(msgJSON) {
        if(!utils.validateMsg(msgJSON, ['content', 'id', 'ts'])) {
            return;
        }

        var msg = JSON.parse(msgJSON);
        var data = JSON.parse(msg['data']);
        var sig = msg['sig'];

        var peerId = data['id'];
        // if(!(peerId in globalConfig['keyTable']) || !utils.verify(globalConfig['keyTable'][peerId], sig, msg['data'])) {
        //     return;
        // }

        if(data['content'] == '' || utils.checkReceivedChat(data)) {
            return;
        }

        utils.addChat(data);
        module.floodReplay(msgJSON);
    }

    return module;
}