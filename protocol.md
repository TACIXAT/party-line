# Network

Nodes use a SHA256 of their public key as their ID on the network.

Nodes store a peer table of 256 peers. Peers are selected for their slot in the peer table based on the closesness of their ID to the ideal ID at `i`. The ideal ID is calculated by `(node_id ^ (1 << i))`, where `i` is in range `[0, 256)`. Whichever peer has the lowest `distance = ideal_id ^ peer_id` get selected for slot `i`. (See: [Kademlia](https://en.wikipedia.org/wiki/Kademlia))

Messages can then be routed to a specific peer by sending the message to the closest peer in the peer table. The receiving peer would then send it to their closest peer, and so on, until it reaches the desired peer. This is `O(log(n))` routing or some nerd shit.

# Bootstrapping

Initial interaction between a node and the node they are bootstrapping to.

**Node A Knows (B's bootstrap info)**

* IP (B)
* PORT (B)
* ID (B) (HASH OF B'S PUBLIC KEY)

**Node A**

1. Generates keypair

2. Generates ID (A) as hash of public key

**Node A - JOIN**

Sends to Node B -

* IP (A)
* PORT (A)
* PUBLIC KEY (A)
* ID (A) (HASH OF A'S PUBLIC KEY)

*Data signed with private key (A).*

**Node B - ON JOIN**

1. Check's signature of message with public key in message

2. Send VERIFY

**Node B - VERIFY**

Send to Node A -

* IP (B)
* PORT (B)
* PUBLIC KEY (B)
* PEER TABLE (B)
* KEY TABLE (B)
* SHA256(PUBLIC KEY A)

*Data signed with private key (B).*

**Node A - ON VERIFY**

1. Check hash of public key in message matches known ID (B)

2. Check signature of message with public key in message

3. Check hash public key (A) in message matches self ID (A)

## Security Assertions

Node A's pre knowledge of Node B's ID (public key hash) allows Node A to confirm Node B's provided public key. It also allows Node A to confirm that their own public key and ID have been successfully received by Node B (someone is not in the middle with different keys).

Someone in the middle would not be able to modify Node B's message without the pre-known ID (B) failing to match.

Someone in the middle would not be able to modify Node A's message without the SHA256(PUBLIC KEY A) verification sent by B becoming incorrect.

## Result

Node A receives Node B's public key, peer table (up to 256 peers for routing throughout the network), and key table (all known peer's keys).

Node A copies Node B's key table. Node A deletes any entry in which the SHA256(KEY) does not match the corresponding ID.

Node A adds Node B to their key table.

Node A uses peers from Node B's peer table to initialize its own peer table. 

# Announce

Announce floods the network to notify that a new peer has joined. A node initializes it by sending the following data to all peers.

**NODE - ANNOUNCE**

* IP
* PORT
* PUBLIC KEY
* ID

*Data signed with private key.*

**PEERS - ON ANNOUNCE**

*If the peers do not have the public key in their key table -*

1. Verify public key hash matches ID provided

2. Verify signature of data with public key

3. Add key to their key table

4. Check peer table and to see if new peer is a better match for any ideal entries, send QUERY_CLOSEST to the new peer if they would fit

5. Forward the signed data to all of their peers

*Else -*

1. Discard the message.

## Security Assertions

**A malicious client could not create fake announce messages to poison routing tables.**

Rather than trusting the announce messages, the node uses `QUERY CLOSEST` which asks the peer for their closest entry to an (in this case, ideal) ID. 

The node then continues this process on each reply until a peer replies that they (self) are the closest. Only peers that reply are updated in the routing table preventing fake nodes from polluting tables. See `QUERY CLOSEST` for more details.

## Result

Network nodes have the new node's public key in their key table for validating messages received from that `ID`. 

Network nodes have initiated a QUERY_CLOSEST to the new node in order to update their peer table when appropriate.

Nodes will not re-propagate the message because the peer has been added to their key table.

# Query Closest

**NODE A - QUERY CLOSEST**

1. Node A stores Node B's `ID` in an array of peer candidates.

2. Node A sends a query to Node B for the closest peer to `TARGET ID`.

* IP (A)
* PORT (A)
* ID (A) 
* TARGET ID

*Data signed with private key (A).*

**NODE B - ON QUERY CLOSEST**

1. Checks Node A ID in key table and data is signed with stored key

2. Locates the closest peer to target id in peer table 

3. Checks if self is closer to target id than the closest from peer table, sets closest to self if true

4. Sends `RESPONSE CLOSEST`

**NODE B - RESPONSE CLOSEST**

Sends response to provided `IP` and `PORT` containing - 

* CLOSEST (PEER ENTRY)
* FROM (PEER ENTRY SELF)
* SELF (BOOLEAN INDICATING THAT SELF IS THE CLOSEST)

*Data signed with private key (B).*

Peer entries are of the form `{id, ip, port, key}`.

**NODE A - ON RESPONSE CLOSEST**

1. Check Node B's key is in key table and `FROM` `ID` in peer candidates

2. Check integrity of message 

3. Check `FROM` key matches key from key table

4. Update peer table with `FROM`

5. If not `SELF`, send `QUERY CLOSEST` to suggested peer

## Security Assertions

**Node B could reply with an incorrect `FROM` info.**

In the first case, they could provide an incorrect `IP` and `PORT` which would mean messages to their ID would be lost (this could be mitigated by checking the IP and PORT a message is received from, but that can be spoofed in UDP, so meh). This will be a minor issue for routing, but is equivalent of a node leaving without a leave message.

In the second case, they could provide a non-existent node in `FROM`, with a fake public key, id, ip, and port. This node would not have an entry in the peer candidates list and the message would be discarded.

## Results

Node A verifies that Node B is active and updates their peer table with Node B's info.

Node A potentially receives a closer candidate than Node B and repeats the process with that Node.

# Leave

**NODE A - LEAVE**

Node A floods a leave message containing - 

* ID
* CLOSEST

*Data signed with private key (A).*

**NODE B - ON LEAVE**

1. Check `ID` in key table

2. Verify message integrity with key from key table

3. Remove the entry from the key table

4. Remove the entry from the peer table if present, replacing any removed entries with existing entries in the peer table

5. For any ideal entry that the peer was removed from, `QUERY CLOSEST` the provided closest peer

6. Flood replay the original (signed) message to peers

## Security Assertions

Due to checking the signature of the leave message received, only nodes holding with the corresponding private key can initiate a leave for their ID.

## Results

Node A is removed from peer's routing tables and key table. 

Node B updates their table to no longer route to Node A.

Node B searches for new nearest nodes.

Nodes will not re-propagate the message because the peer has been deleted from their key table.