var crypto = require('crypto');

module.exports = function(globalConfig, net, utils) {
    var name = '';
    var channelSecret;
    var peers = [];
    // peer = {nick: '', id: ''}
    var invites = [];
    // invite = {invite: invite, channel: name, type: id/passphrase}
    var messages = [];
    
    module.init(name, channelSecret) {
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
        var nameBuf = Buffer.from(name);
        var keyBuf = Buffer.from(key);
        var invite = {
            code: util.sha256(Buffer.concat([channelSecret, nameBuf, keyBuf])),
            name: name,
            type: type,
        }
    }

    module.checkInvite = function(invite) {
        var keyBuf = Buffer.from(invite['key']);
        var nameBuf = Buffer.from(invite['name']);
        var code = util.sha256(Buffer.concat([channelSecret, nameBuf, keyBuf]));
        if(code == invite['code']) {
            // success
        }
    }

    return module;
}