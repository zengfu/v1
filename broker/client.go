package broker

import (
	"errors"
	"fmt"
	"github.com/gomqtt/packet"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"net"
	"time"
)

type Subscribe struct {
	ID       uint `gorm:"primary_key"`
	ClientID uint
	// The topic to subscribe.
	Topic string `gorm:"size:65535;not null`
	// The requested maximum QOS level.
	QOS uint8 `gorm:"not null"`
}
type Message struct {
	ID       uint `gorm:"primary_key"`
	ClientID uint
	// The Topic of the message.
	Topic string `gorm:"size:65535"`
	// The Payload of the message.
	Payload string `gorm:"size:65535"`
	// The QOS indicates the level of assurance for delivery.
	QOS byte
	// If the Retain flag is set to true, the server must store the message,
	// so that it can be delivered to future subscribers whose subscriptions
	// match its topic name.
	Retain bool
}
type Client struct {
	ID uint `gorm:"primary_key"`
	// The clients client id.
	ClientID string `gorm:"size:65535;not null;unique_index`
	// The keep alive value.
	KeepAlive uint16 `gorm:"not null"`
	// The authentication username.
	Username string `gorm:"size:65535"`
	// The authentication password.
	Password string `gorm:"size:65535"`
	// The clean session flag.
	CleanSession bool `gorm:"not null"`
	// The Online status
	Online bool `gorm:"not null"`
	// The will message.
	Will Message `gorm:"ForeignKey:ClientID"`
	// The MQTT version 3 or 4 (defaults to 4 when 0).
	Version    byte           `gorm:"not null"`
	Subscribes []Subscribe    `gorm:"ForeignKey:ClientID"`
	socket     net.Conn       `gorm:"-"`
	stream     *packet.Stream `gorm:"-"`
}

func NewClient(conn net.Conn) *Client {

	return &Client{
		socket: conn,
		stream: packet.NewStream(conn, conn),
	}
}
func (c *Client) prcessConnect(connect *packet.ConnectPacket) error {
	connack := packet.NewConnackPacket()
	c.KeepAlive = connect.KeepAlive
	c.Online = true
	c.Password = connect.Password
	c.ClientID = connect.ClientID
	c.Username = connect.Username
	c.Version = connect.Version
	c.CleanSession = connect.CleanSession
	if connect.Will != nil {
		c.Will.Payload = string(connect.Will.Payload)
		c.Will.QOS = connect.Will.QOS
		c.Will.Topic = connect.Will.Topic
	}
	indb, err := c.SaveDb()
	fmt.Println(indb, err)
	fmt.Println(c.ID, c.ClientID)
	if err != nil {
		connack.ReturnCode = 0x02
		connack.SessionPresent = false
	} else {
		connack.ReturnCode = 0
		if indb && c.CleanSession == false {
			connack.SessionPresent = true
		} else {
			connack.SessionPresent = false
		}
	}
	c.stream.Encoder.Write(connack)
	c.stream.Flush()
	if connack.ReturnCode > 0 {
		a := fmt.Sprintf("returncode%v", connack.ReturnCode)
		return errors.New(a)
	} else {
		return err
	}

}
func (c *Client) Process() {
	defer c.socket.Close()
	fmt.Println(c.socket.RemoteAddr())
	c.socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	stream := packet.NewStream(c.socket, c.socket)
	first := true
	for {
		pkt, err := stream.Decoder.Read()
		if err != nil {
			fmt.Println(err)
			break
		}
		if first {
			connect, ok := pkt.(*packet.ConnectPacket)
			if !ok {
				fmt.Println("expected connect")
				break
			}
			err = c.prcessConnect(connect)
			if err != nil {
				fmt.Println(err)
				return
			}
			first = false
			continue
		}
		c.socket.SetReadDeadline(time.Now().Add(time.Duration(float32(c.KeepAlive)*1.5) * time.Second))

		switch _pkt := pkt.(type) {
		case *packet.SubscribePacket:
			err = c.processSubscribe(_pkt)
		case *packet.PingreqPacket:
			err = c.processPingreq(_pkt)
		case *packet.UnsubscribePacket:
			err = c.processUnsubscribe(_pkt)
			/*
				case *PublishPacket:
					err = s.processPublic(stream, _pkt)
				case *PubackPacket:
					err = s.processPuback(stream, _pkt)
				case *PubrecPacket:
					err = s.processPubrec(stream, _pkt)
				case *PubrelPacket:
					err = s.processPubrel(stream, _pkt)
				case *PubcompPacket:
					err = s.processPubcomp(stream, _pkt)
				case *PingreqPacket:
					err = s.processPingreq(stream, _pkt)
				case *DisconnectPacket:
					err = s.processDisconnect(stream, _pkt)*/
		default:
			err = errors.New("error packet type")
		}
		if err != nil {
			fmt.Println(err)
			break
		}
	}
	c.RealeasDb()
	fmt.Println("socket close")

}
func (c *Client) processUnsubscribe(pkt *packet.UnsubscribePacket) error {
	//	unsuback := packet.NewUnsubackPacket()
	//	unsuback.PacketID = pkt.PacketID
	return nil

}
func (c *Client) processPingreq(pkt *packet.PingreqPacket) error {
	if err := c.stream.Write(packet.NewPingrespPacket()); err != nil {
		return err
	}
	if err := c.stream.Flush(); err != nil {
		return err
	}
	return nil
}
func (c *Client) processSubscribe(pkt *packet.SubscribePacket) error {
	suback := packet.NewSubackPacket()
	suback.ReturnCodes = make([]byte, len(pkt.Subscriptions))
	suback.PacketID = pkt.PacketID

	for i, subscription := range pkt.Subscriptions {
		err := c.SaveSub(subscription)
		if err != nil {
			return err
		}
		//subscribe to queue
		suback.ReturnCodes[i] = subscription.QOS
	}
	if err := c.stream.Write(suback); err != nil {
		return err
	}

	if err := c.stream.Flush(); err != nil {
		return err
	}
	//queue tht retained messages
	return nil
}
func (c *Client) DeleteSub(topic string) error {
	db, err := OpenDb()
	if err != nil {
		return err
	}
	defer db.Close()
	var tmpc Client
	var sublist []Subscribe

	db.Where("client_id=?", c.ClientID).First(&tmpc)
	copy(sublist, tmpc.Subscribes)
	tmpc.Subscribes = nil
	for _, sub := range sublist {
		if sub.Topic != topic {
			tmpc.Subscribes = append(tmpc.Subscribes, sub)
		}
	}
	db.Save(&tmpc)
	return nil
}
func (c *Client) SaveSub(asub packet.Subscription) error {
	db, err := OpenDb()
	if err != nil {
		return err
	}
	defer db.Close()
	var tmpc Client
	var subscibe Subscribe
	subscibe.QOS = asub.QOS
	subscibe.Topic = asub.Topic

	db.Where("client_id=?", c.ClientID).First(&tmpc)
	var subscribes []Subscribe
	db.Model(&tmpc).Related(&subscribes, "Subscribes")
	for _, subtopic := range subscribes {
		if subtopic.Topic == subscibe.Topic {
			subtopic.QOS = subscibe.QOS
			db.Save(&subtopic)
			return nil
		}
	}
	tmpc.Subscribes = append(tmpc.Subscribes, subscibe)
	db.Save(&tmpc)
	return nil
}
func (c *Client) RealeasDb() error {
	db, err := OpenDb()
	if err != nil {
		return err
	}
	defer db.Close()
	var tmpc Client
	db.Where("client_id=?", c.ClientID).First(&tmpc)
	if tmpc.CleanSession == false {
		tmpc.Online = false
		db.Save(&tmpc)
		return nil
	}
	var subscribes []Subscribe
	var message Message
	db.Model(&tmpc).Related(&subscribes, "Subscribes")
	db.Model(&tmpc).Related(&message)
	fmt.Println(message)
	db.Delete(&message)
	for _, subtopic := range subscribes {
		fmt.Println(subtopic)
		db.Delete(&subtopic)
	}
	db.Delete(&tmpc)
	fmt.Println("release database")
	return nil
}
func (c *Client) SaveDb() (bool, error) {
	db, err := OpenDb()
	indb := false
	if err != nil {
		return indb, err
	}
	defer db.Close()
	var tmpc Client
	if db.Where("client_id=?", c.ClientID).First(&tmpc).RecordNotFound() {
		indb = false
		db.Create(c)
	} else {
		indb = true
		if tmpc.Online {
			return false, errors.New("the clientid is using")
		} else {
			if c.CleanSession {
				tmpc.RealeasDb()
				db.Create(c)
			} else {
				c.ID = tmpc.ID
				tmpc.KeepAlive = c.KeepAlive
				tmpc.Username = c.Username
				tmpc.Password = c.Password
				tmpc.Will.Payload = c.Will.Payload
				tmpc.Will.QOS = c.Will.QOS
				tmpc.Will.Topic = c.Will.Topic
				db.Save(&tmpc)

			}
		}
	}
	return indb, err
}
