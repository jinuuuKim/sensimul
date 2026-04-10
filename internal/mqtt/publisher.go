package mqtt

import (
	"context"
	"encoding/json"
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

	topic := TopicLiveSensor(siteID, sensorID)
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

func (p *Publisher) PublishTestRequest(ctx context.Context, req SensorTestRequest) error {
	if p == nil {
		return nil
	}
	if p.client == nil || !p.client.IsConnectionOpen() {
		return fmt.Errorf("mqtt not connected")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal test request: %w", err)
	}

	topic := TopicTestRequest(req.SiteID, req.SensorID)
	token := p.client.Publish(topic, p.opts.QoS, false, body)
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

func (p *Publisher) PublishTestResult(ctx context.Context, result SensorTestResult) error {
	if p == nil {
		return nil
	}
	if p.client == nil || !p.client.IsConnectionOpen() {
		return fmt.Errorf("mqtt not connected")
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal test result: %w", err)
	}

	topic := TopicTestResult(result.SiteID, result.SensorID)
	token := p.client.Publish(topic, p.opts.QoS, false, body)
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

func (p *Publisher) Subscribe(ctx context.Context, topic string, handler func(topic string, payload []byte)) error {
	if p == nil {
		return nil
	}
	if p.client == nil || !p.client.IsConnectionOpen() {
		return fmt.Errorf("mqtt not connected")
	}

	token := p.client.Subscribe(topic, p.opts.QoS, func(_ paho.Client, msg paho.Message) {
		handler(msg.Topic(), msg.Payload())
	})
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt subscribe timeout: %s", topic)
	}
	if err := token.Error(); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		ut := p.client.Unsubscribe(topic)
		ut.WaitTimeout(3 * time.Second)
	}()

	return nil
}

func (p *Publisher) Close() {
	if p == nil || p.client == nil {
		return
	}
	p.client.Disconnect(200)
	p.logger.Info().Msg("mqtt disconnected")
}
