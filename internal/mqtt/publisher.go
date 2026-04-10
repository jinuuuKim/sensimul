package mqtt

import (
	"context"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
)

// Options captures runtime MQTT connection and publish behavior.
type Options struct {
	BrokerURL string
	ClientID  string
	QoS       byte
	Retain    bool
}

// Publisher wraps MQTT client lifecycle and topic conventions.
type Publisher struct {
	client paho.Client
	opts   Options
	logger zerolog.Logger
}

func NewPublisher(opts Options, logger zerolog.Logger) *Publisher {
	return &Publisher{
		opts:   opts,
		logger: logger,
	}
}

func (p *Publisher) Connect(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if p.opts.BrokerURL == "" {
		return fmt.Errorf("mqtt broker_url is empty")
	}
	if p.opts.ClientID == "" {
		p.opts.ClientID = fmt.Sprintf("sensimul-%d", time.Now().UnixNano())
	}

	options := paho.NewClientOptions().
		AddBroker(p.opts.BrokerURL).
		SetClientID(p.opts.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true)

	p.client = paho.NewClient(options)
	token := p.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p.logger.Info().Str("broker", p.opts.BrokerURL).Msg("mqtt connected")
	return nil
}

func (p *Publisher) PublishSensor(ctx context.Context, siteID, sensorID string, payload []byte) error {
	if p == nil {
		return nil
	}
	if p.client == nil || !p.client.IsConnectionOpen() {
		return fmt.Errorf("mqtt not connected")
	}

	topic := fmt.Sprintf("sensimul/sites/%s/sensors/%s", siteID, sensorID)
	token := p.client.Publish(topic, p.opts.QoS, p.opts.Retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt publish timeout: %s", topic)
	}
	if err := token.Error(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

func (p *Publisher) Close() {
	if p == nil || p.client == nil {
		return
	}
	p.client.Disconnect(200)
	p.logger.Info().Msg("mqtt disconnected")
}
