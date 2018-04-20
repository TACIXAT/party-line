var crypto = require('crypto');

module.exports = function(globalConfig, net, utils) {
    var name = '';
    var channelSecret;
    var peers = [];
    // peer = {nick: '', id: ''}
    var invites = [];
    // invite = {invite: invite, channel: name, type: id/passphrase}
    var messages = [];
    
    module.init = function(name, channelSecret) {
        if(!channelSecret) {
            this.name = `${name}.${globalConfig['id'].substr(64-6, 6)}`;
            this.channelSecret = crypto.randomBytes(32);
        } else {
            this.name = name;
            this.channelSecret = channelSecret;
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

    module.createInvite = function(type, key) {
        // hash(channelSecret + name + key)
        // type is 'id' or 'pass'
        var nameBuf = Buffer.from(this.name);
        var keyBuf = Buffer.from(key);
        var typeBuf = Buffer.from(type);
        var invite = {
            code: utils.sha256(Buffer.concat([this.channelSecret, nameBuf, typeBuf, keyBuf])),
            name: this.name,
            type: type,
        }
        return invite;
    }

    module.checkInvite = function(invite) {
        var keyBuf = Buffer.from(invite['key']);
        var nameBuf = Buffer.from(this.name);
        var typeBuf = Buffer.from(invite['type']);
        var code = utils.sha256(Buffer.concat([this.channelSecret, nameBuf, typeBuf, keyBuf]));
        if(code == invite['code']) {
            return true;
        }

        return false;
    }

    module.sendInvite = function(id, type, key) {
        // send an invite to a user
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