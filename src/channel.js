var crypto = require('crypto');

module.exports = function(globalConfig, net, utils) {
    module.init = function(name, channelSecret, peers) {
        this.peers = [];
        this.messages = [];
        this.name = '';
        
        if(!channelSecret) {
            this.name = `${name}.${globalConfig['id'].substr(64-6, 6)}`;
            this.channelSecret = crypto.randomBytes(32);
        } else {
            this.name = name;
            this.channelSecret = channelSecret;
            if(peers) {
                this.peers = peers;
                module.announce();
            }
        }
    }

    module.sendMessage = function(msg) {
        var b64 = utils.encryptMessage(channelSecret, msg);
        var data = {
            type: 'channel',
            name: name,
            id: globalConfig['id'],
            message: b64,
        }

        // send all participants
        for(var idx in peers) {
            var peer = peers[idx];
            net.route(peer, globalConfig['pair'], data);
        }
    }

    module.sendInvite = function(peerId) {
        if(!(peerId in globalConfig['secretTable'])) {
            return false;
        }

        // make a copy?        
        var channelPeerList = this.peers.map(function(ea) { return ea; });
        channelPeerList.push(globalConfig['id']);
        var invite = {
            secret: this.channelSecret.toString('hex'),
            peers: channelPeerList,
            name: this.name,
        };

        var b64 = utils.encryptMessage(globalConfig['secretTable'][peerId], JSON.stringify(invite));

        var data = {
            type: 'private_invite',
            enc: b64,
            target: peerId,
            id: globalConfig['id'],
            ts: Date.now(),
        }
        net.route(peerId, globalConfig['pair'], data);
    }

    module.announce = function() {
        // announce self to channel
    }

    module.onAnnounce = function(pair, msgJSON) {
        // add peer to peer list
    }

    module.sendMessage = function() {
        // send message to peer list
    }

    module.onMessage = function() {
        // display message then forward
    }

    module.propagateMessage = function() {
        // forward message to peers
    }

    return module;
}