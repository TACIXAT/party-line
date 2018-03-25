var crypto = require('crypto');

module.exports = function(name, id) {
    var name = name;
    var creator = id;
    var peers = [];
    // peer = {nick: '', id: ''}
    var channelSecret = crypto.randomBytes(32);
    var invites = [];
    // invite = {invite: invite, channel: name, type: id/passphrase}
    var messages = [];
    var algorithm = 'AES-256-CFB';

    module.decryptMessage = function(msg) {
        // b64 decode
        var enc = Buffer.from(msg, 'base64');
        var iv = enc.slice(0, 16);
        var enc = enc.slice(16);
        // decrypt with channel secret
        var decipher = crypto.createDecipheriv(algorithm, channelSecret, iv);
        var dec = Buffer.concat([decipher.update(enc), decipher.final()]);
        return dec.toString();
    }

    module.sendMessage = function(msg) {
        // encrypt with channel secret
        var iv = crypto.randomBytes(16);
        var cipher = crypto.createCipheriv(algorithm, channelSecret, iv);
        var enc = cipher.update(msg);
        // b64 encode
        enc = Buffer.concat([enc, cipher.final()]);
        enc = Buffer.concat([iv, enc]);
        var b64 = enc.toString('base64');
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