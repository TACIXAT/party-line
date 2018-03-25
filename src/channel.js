var crypto = require('crypto');

module.exports = function(name, id, utils) {
    var name = name;
    var creator = id;
    var peers = [];
    // peer = {nick: '', id: ''}
    var channelSecret = crypto.randomBytes(32);
    var invites = [];
    // invite = {invite: invite, channel: name, type: id/passphrase}
    var messages = [];

    module.sendMessage = function(msg) {
        var b64 = utils.encryptMessage(channelSecret, msg);
        // send all participants
        return b64;
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