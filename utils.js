var crypto = require('crypto');

module.exports = function(globalConfig, net) {

    module.sha256 = function(data) {
        return crypto.createHash('sha256').update(data).digest('hex');
    }

    module.sign = function(pair, data) {
        var signer = crypto.createSign('SHA256');
        signer.update(data);

        var privkey = pair.private;
        var sig = signer.sign(privkey, 'hex');
        return sig;
    }

    module.verify = function(pubkey, sig, data) {
        var verifier = crypto.createVerify('sha256');
        verifier.update(data);
        return verifier.verify(pubkey, sig, 'hex');
    }

    module.bufferXor = function(a, b) {
        var length = Math.max(a.length, b.length)
        var buffer = Buffer.allocUnsafe(length)

        for (var i = 0; i < length; ++i) {
             buffer[i] = a[i] ^ b[i];
        }

        return buffer
    }

    module.calculateIdealRoutingTable = function(id) {
        var idealPeerList = [];
        var idBuf = Buffer.from(id, 'hex');
        for(var i = 0; i < 256; i++) {
            var powerBuf = Buffer.from((2**i).toString(16).padStart(64, '0'), 'hex');
            var idealPeer = module.bufferXor(id, powerBuf);
            idealPeerList.push(idealPeer.toString('hex'));
        }
        return idealPeerList;
    }

    module.delayQueryClosest = function(pair, peer, targetId) {
        net.queryClosest(pair, peer, targetId);
    }

    // build node routing table
    module.buildRoutingTable = function(pair) {
        console.log('building routing table...');
        for(var i = 0; i < 256; i++) {
            globalConfig['peerTable'][i] = globalConfig['bootstrapPeer'];
        }

        console.log('announcing to network...');
        net.announce(pair);

        console.log('querying peers for closest...');
        var peer = globalConfig['bootstrapPeer'];
        for(var i = 0; i < 256; i++) {
            var targetId = globalConfig['idealRoutingTable'][i];
            setTimeout(module.delayQueryClosest.bind(undefined, pair, peer, targetId), 20 * (i+1));
        }

        setTimeout(function() {
            console.log('happy chatting!');
        }, 20 * 258);
    }

    module.wouldUpdateTable = function(peer) {
        var idealMatches = [];
        for(var i = 0; i < 256; i++) {
            var targetId = globalConfig['idealRoutingTable'][i];
            var currPeer = globalConfig['peerTable'][i];
            
            if(currPeer === null || currPeer === undefined) {
                idealMatches.push(targetId);
                continue;
            }
            
            var targetIdBuf = Buffer.from(targetId, 'hex');
            var currIdBuf = Buffer.from(currPeer['id'], 'hex');
            var peerIdBuf = Buffer.from(peer['id'], 'hex');

            var currDistance = module.bufferXor(targetIdBuf, currIdBuf);
            var peerDistance = module.bufferXor(targetIdBuf, peerIdBuf);

            if(peerDistance.compare(currDistance) < 0) {
                idealMatches.push(targetId);
            }
        }
        return idealMatches;
    }

    module.updateTable = function(peer) {
        var idealMatches = [];
        for(var i = 0; i < 256; i++) {
            var targetId = globalConfig['idealRoutingTable'][i];
            var currPeer = globalConfig['peerTable'][i];

            if(currPeer === null || currPeer === undefined) {
                idealMatches.push(targetId);
                globalConfig['peerTable'][i] = peer;
                continue;
            }

            var targetIdBuf = Buffer.from(targetId, 'hex');
            var currIdBuf = Buffer.from(currPeer['id'], 'hex');
            var peerIdBuf = Buffer.from(peer['id'], 'hex');

            var currDistance = module.bufferXor(targetIdBuf, currIdBuf);
            var peerDistance = module.bufferXor(targetIdBuf, peerIdBuf);

            if(peerDistance.compare(currDistance) < 0) {
                idealMatches.push(targetId);
                globalConfig['peerTable'][i] = peer;
            }
        }

        return idealMatches;
    }

    module.findClosestExclude = function(searchTable, targetId, [excludeIds]) {
        var targetIdBuf = Buffer.from(targetId, 'hex');

        var closest;
        for(var idx in searchTable) {
            if(!searchTable[idx] || excludeIds.indexOf(searchTable[idx]['id']) > -1) {
                continue;
            }
            closest = searchTable[idx];
            break;
        }

        if(closest === undefined || closest === null) {
            return undefined;
        }

        var closestIdBuf = Buffer.from(closest['id'], 'hex');
        var closestDist = module.bufferXor(closestIdBuf, targetIdBuf);

        for(var idx in searchTable) {
            var peer = searchTable[idx];

            if(!peer || excludeIds.indexOf(peer['id']) > -1) {
                continue;
            }

            var peerId = peer['id'];
            var peerIdBuf = Buffer.from(peerId, 'hex');
            var peerDist = module.bufferXor(peerIdBuf, targetIdBuf);
            if(peerDist.compare(closestDist) < 0) {
                closest = peer;
                closestDist = peerDist;
            }
        }

        return closest;
    }

    module.updateTableRemove = function(peerId) {
        var contains = false;
        var indices = [];
        for(var i = 0; i < 256; i++) {
            if(globalConfig['peerTable'][i]['id'] === peerId) {
                indices.push(i);
            }
        }

        if(indices.length === 0) {
            return;
        }

        var idealIds = [];
        for(var j in indices) {
            var idx = indices[j];
            var peer = globalConfig['peerTable'][idx];
            var idealPeerId = globalConfig['idealRoutingTable'][idx];
            idealIds.push(idealPeerId);
            var closest = module.findClosestExclude(globalConfig['peerTable'], idealPeerId, [peer['id']]);
            
            if(closest === undefined) {
                delete globalConfig['peerTable'][idx];
                continue;
            }

            globalConfig['peerTable'][idx] = closest;
        }
        return idealIds;
    }

    module.findClosest = function(searchTable, targetId) {
        var targetIdBuf = Buffer.from(targetId, 'hex');

        var closest = searchTable[0];
        
        if(!closest) {
            return closest;
        }

        var closestIdBuf = Buffer.from(closest['id'], 'hex');
        var closestDist = module.bufferXor(closestIdBuf, targetIdBuf);

        for(var idx in searchTable) {
            var peer = searchTable[idx];
            var peerId = peer['id'];
            var peerIdBuf = Buffer.from(peerId, 'hex');
            var peerDist = module.bufferXor(peerIdBuf, targetIdBuf);
            if(peerDist.compare(closestDist) < 0) {
                closest = peer;
                closestDist = peerDist;
            }
        }

        return closest;
    }

    module.checkReceivedChat = function(chat) {
        var peerId = chat['id'];
        var chatTs = chat['ts'];
        if(!(peerId in globalConfig['chatMessagesReceived'])) {
            globalConfig['chatMessagesReceived'][peerId] = [];
        }

        if(globalConfig['chatMessagesReceived'][peerId].indexOf(chatTs) < 0) {
            globalConfig['chatMessagesReceived'][peerId].push(chatTs);
            return false;
        } 
          
        return true;
    }

    module.validateMsg = function(msgJSON, dataKeys) {
        var msgKeys = ['sig', 'data'];
        if(!module.isJSON(msgJSON)) {
            console.log('discarding', msgJSON.toString());
            return false;
        }

        var msg = JSON.parse(msgJSON);
        for(var idx in msgKeys) {
            var key = msgKeys[idx];
            if(!(key in msg)) {
            console.log('discarding', msgJSON.toString());
                return false;
            }
        }

        if(!module.isJSON(msg['data'])) {
            console.log('discarding', msgJSON.toString());
            return false;
        }

        var data = JSON.parse(msg['data']);
        for(var idx in dataKeys) {
            var key = dataKeys[idx];
            if(!(key in data)) {
                console.log('discarding', msgJSON.toString());
                return false;
            }   
        }

        return true;
    }

    module.isJSON = function(item) {
        item = typeof item !== "string"
            ? JSON.stringify(item)
            : item;

        try {
            item = JSON.parse(item);
        } catch (e) {
            return false;
        }

        if (typeof item === "object" && item !== null) {
            return true;
        }

        return false;
    }

    module.addChat = function(data) {
        var chat = {
            id: data['id'],
            ts: data['ts'],
            content: data['content'],
            time: Date.now(),
        }

        globalConfig['chatMessages'].push(chat);
        globalConfig['chatMessages'].shift();
        console.log(`${data['id'].substr(64-6, 6)}: ${data['content']}`);
    }

    return module;
}