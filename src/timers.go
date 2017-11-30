package main

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"sync/atomic"
	"time"
)

/* Called when a new authenticated message has been send
 *
 */
func (peer *Peer) KeepKeyFreshSending() {
	kp := peer.keyPairs.Current()
	if kp == nil {
		return
	}
	nonce := atomic.LoadUint64(&kp.sendNonce)
	if nonce > RekeyAfterMessages {
		peer.signal.handshakeBegin.Send()
	}
	if kp.isInitiator && time.Now().Sub(kp.created) > RekeyAfterTime {
		peer.signal.handshakeBegin.Send()
	}
}

/* Called when a new authenticated message has been received
 *
 * NOTE: Not thread safe (called by sequential receiver)
 */
func (peer *Peer) KeepKeyFreshReceiving() {
	if peer.timer.sendLastMinuteHandshake {
		return
	}
	kp := peer.keyPairs.Current()
	if kp == nil {
		return
	}
	if !kp.isInitiator {
		return
	}
	nonce := atomic.LoadUint64(&kp.sendNonce)
	send := nonce > RekeyAfterMessages || time.Now().Sub(kp.created) > RekeyAfterTimeReceiving
	if send {
		// do a last minute attempt at initiating a new handshake
		peer.signal.handshakeBegin.Send()
		peer.timer.sendLastMinuteHandshake = true
	}
}

/* Queues a keep-alive if no packets are queued for peer
 */
func (peer *Peer) SendKeepAlive() bool {
	elem := peer.device.NewOutboundElement()
	elem.packet = nil
	if len(peer.queue.nonce) == 0 {
		select {
		case peer.queue.nonce <- elem:
			return true
		default:
			return false
		}
	}
	return true
}

/* Event:
 * Sent non-empty (authenticated) transport message
 */
func (peer *Peer) TimerDataSent() {
	peer.timer.keepalivePassive.Stop()
	if peer.timer.newHandshake.Pending() {
		peer.timer.newHandshake.Reset(NewHandshakeTime)
	}
}

/* Event:
 * Received non-empty (authenticated) transport message
 *
 * Action:
 * Set a timer to confirm the message using a keep-alive (if not already set)
 */
func (peer *Peer) TimerDataReceived() {
	if !peer.timer.keepalivePassive.Start(KeepaliveTimeout) {
		peer.timer.needAnotherKeepalive = true
	}
}

/* Event:
 * Any (authenticated) packet received
 */
func (peer *Peer) TimerAnyAuthenticatedPacketReceived() {
	peer.timer.newHandshake.Stop()
}

/* Event:
 * Any authenticated packet send / received.
 *
 * Action:
 * Push persistent keep-alive into the future
 */
func (peer *Peer) TimerAnyAuthenticatedPacketTraversal() {
	interval := atomic.LoadUint64(&peer.persistentKeepaliveInterval)
	if interval > 0 {
		duration := time.Duration(interval) * time.Second
		peer.timer.keepalivePersistent.Reset(duration)
	}
}

/* Called after successfully completing a handshake.
 * i.e. after:
 *
 * - Valid handshake response
 * - First transport message under the "next" key
 */
func (peer *Peer) TimerHandshakeComplete() {
	atomic.StoreInt64(
		&peer.stats.lastHandshakeNano,
		time.Now().UnixNano(),
	)
	peer.signal.handshakeCompleted.Send()
	peer.device.log.Info.Println("Negotiated new handshake for", peer.String())
}

/* Event:
 * An ephemeral key is generated
 *
 * i.e. after:
 *
 * CreateMessageInitiation
 * CreateMessageResponse
 *
 * Action:
 * Schedule the deletion of all key material
 * upon failure to complete a handshake
 */
func (peer *Peer) TimerEphemeralKeyCreated() {
	peer.timer.zeroAllKeys.Reset(RejectAfterTime * 3)
}

func (peer *Peer) RoutineTimerHandler() {
	device := peer.device

	logInfo := device.log.Info
	logDebug := device.log.Debug
	logDebug.Println("Routine, timer handler, started for peer", peer.String())

	for {
		select {

		/* timers */

		// keep-alive

		case <-peer.timer.keepalivePersistent.Wait():

			interval := atomic.LoadUint64(&peer.persistentKeepaliveInterval)
			if interval > 0 {
				logDebug.Println("Sending keep-alive to", peer.String())
				peer.SendKeepAlive()
			}

		case <-peer.timer.keepalivePassive.Wait():

			logDebug.Println("Sending keep-alive to", peer.String())

			peer.SendKeepAlive()

			if peer.timer.needAnotherKeepalive {
				peer.timer.keepalivePassive.Reset(KeepaliveTimeout)
				peer.timer.needAnotherKeepalive = false
			}

		// clear key material timer

		case <-peer.timer.zeroAllKeys.Wait():

			logDebug.Println("Clearing all key material for", peer.String())

			hs := &peer.handshake
			hs.mutex.Lock()

			kp := &peer.keyPairs
			kp.mutex.Lock()

			// remove key-pairs

			if kp.previous != nil {
				device.DeleteKeyPair(kp.previous)
				kp.previous = nil
			}
			if kp.current != nil {
				device.DeleteKeyPair(kp.current)
				kp.current = nil
			}
			if kp.next != nil {
				device.DeleteKeyPair(kp.next)
				kp.next = nil
			}
			kp.mutex.Unlock()

			// zero out handshake

			device.indices.Delete(hs.localIndex)

			hs.localIndex = 0
			setZero(hs.localEphemeral[:])
			setZero(hs.remoteEphemeral[:])
			setZero(hs.chainKey[:])
			setZero(hs.hash[:])
			hs.mutex.Unlock()

		// handshake timers

		case <-peer.timer.newHandshake.Wait():
			logInfo.Println("Retrying handshake with", peer.String())
			peer.signal.handshakeBegin.Send()

		case <-peer.timer.handshakeTimeout.Wait():

			// clear source (in case this is causing problems)

			peer.mutex.Lock()
			if peer.endpoint != nil {
				peer.endpoint.ClearSrc()
			}
			peer.mutex.Unlock()

			// send new handshake

			err := peer.sendNewHandshake()
			if err != nil {
				logInfo.Println(
					"Failed to send handshake to peer:", peer.String())
			}

		case <-peer.timer.handshakeDeadline.Wait():

			// clear all queued packets and stop keep-alive

			logInfo.Println(
				"Handshake negotiation timed out for:", peer.String())

			peer.signal.flushNonceQueue.Send()
			peer.timer.keepalivePersistent.Stop()
			peer.signal.handshakeBegin.Enable()

		/* signals */

		case <-peer.signal.stop.Wait():
			return

		case <-peer.signal.handshakeBegin.Wait():

			peer.signal.handshakeBegin.Disable()

			err := peer.sendNewHandshake()
			if err != nil {
				logInfo.Println(
					"Failed to send handshake to peer:", peer.String())
			}

			peer.timer.handshakeDeadline.Reset(RekeyAttemptTime)

		case <-peer.signal.handshakeCompleted.Wait():

			logInfo.Println(
				"Handshake completed for:", peer.String())

			peer.timer.handshakeTimeout.Stop()
			peer.timer.handshakeDeadline.Stop()
			peer.signal.handshakeBegin.Enable()
		}
	}
}

/* Sends a new handshake initiation message to the peer (endpoint)
 */
func (peer *Peer) sendNewHandshake() error {

	// temporarily disable the handshake complete signal

	peer.signal.handshakeCompleted.Disable()

	// create initiation message

	msg, err := peer.device.CreateMessageInitiation(peer)
	if err != nil {
		return err
	}

	// marshal handshake message

	var buff [MessageInitiationSize]byte
	writer := bytes.NewBuffer(buff[:0])
	binary.Write(writer, binary.LittleEndian, msg)
	packet := writer.Bytes()
	peer.mac.AddMacs(packet)

	// send to endpoint

	err = peer.SendBuffer(packet)
	if err == nil {
		peer.TimerAnyAuthenticatedPacketTraversal()
		peer.signal.handshakeCompleted.Enable()
	}

	// set timeout

	jitter := time.Millisecond * time.Duration(rand.Uint32()%334)
	peer.timer.handshakeTimeout.Reset(RekeyTimeout + jitter)

	return err
}