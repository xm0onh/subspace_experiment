package operator

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/anthdm/hollywood/log"
	"github.com/xm0onh/subspace_experiment/config"
	sLog "github.com/xm0onh/subspace_experiment/log"
)

type handler struct {
}

func (handler) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case []byte:
		fmt.Println("got message to handle:", string(msg))

	case actor.Stopped:
		for i := 0; i < 10; i++ {
			fmt.Printf("\r handler stopping in %d", 3-i)
			time.Sleep(time.Second)
		}
		fmt.Println("handler stopped")
	}
}

type session struct {
	conn     net.Conn
	msg      chan []byte
	operator *operator
}

type connAdd struct {
	pid  *actor.PID
	conn net.Conn
}

type connRem struct {
	pid *actor.PID
}

type server struct {
	listenAddr string
	ln         net.Listener
	sessions   map[*actor.PID]net.Conn
	operator   *operator
}

func (s *session) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Initialized:
	case actor.Started:
		log.Infow("new connection", log.M{"addr": s.conn.RemoteAddr()})
		go s.readLoop(c)
	case actor.Stopped:
		s.conn.Close()
	}
}

func (s *session) readLoop(c *actor.Context) {
	buf := make([]byte, 1024)
	for {
		n, err := s.conn.Read(buf)
		if err != nil {
			log.Errorw("conn read error", log.M{"err": err})
			break
		}
		// copy shared buffer, to prevent race conditions.
		msg := make([]byte, n)

		copy(msg, buf[:n])
		s.operator.test = string(msg)
		// fmt.Println(<-s.operator.test)
		// fmt.Println("-------->", msg)
		// Send to the handler to process to message
		// c.Send(c.Parent().Child("handler"), msg)
	}
	// Loop is done due to error or we need to close due to server shutdown.
	c.Send(c.Parent(), &connRem{pid: c.PID()})
}

func (s *server) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Initialized:
		ln, err := net.Listen("tcp", s.listenAddr)
		if err != nil {
			panic(err)
		}
		s.ln = ln
		// start the handler that will handle the incomming messages from clients/sessions.
		c.SpawnChild(s.operator.newHandler, "handler")
	case actor.Started:
		log.Infow("server started", log.M{"addr": s.listenAddr})
		go s.acceptLoop(c)
	case actor.Stopped:
		// on stop all the childs sessions will automatically get the stop
		// message and close all their underlying connection.
	case *connAdd:
		log.Tracew("added new connection to my map", log.M{"addr": msg.conn.RemoteAddr(), "pid": msg.pid})
		s.sessions[msg.pid] = msg.conn
	case *connRem:
		log.Tracew("removed connection from my map", log.M{"pid": msg.pid})
		delete(s.sessions, msg.pid)
	}
}

// done
func (s *server) acceptLoop(c *actor.Context) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Errorw("accept error", log.M{"err": err})
			break
		}
		pid := c.SpawnChild(s.operator.newSession(conn), "session", actor.WithTags(conn.RemoteAddr().String()))
		c.Send(c.PID(), &connAdd{
			pid:  pid,
			conn: conn,
		})
	}
}

func (o *operator) http() {
	ip, err := url.Parse(config.Configuration.HTTPAddrs[o.id])
	if err != nil {
		sLog.Fatal("http url parse error: ", err)
	}
	port := ":" + ip.Port()
	fmt.Println(port)
	listenAddr := flag.String("listenaddr"+fmt.Sprint(o.id), port, "listen address of the TCP server")
	sLog.Info("Node ", o.id, " http server starting on ", port)
	e := actor.NewEngine()
	serverPID := e.Spawn(o.newServer(*listenAddr), "server")

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	<-sigch

	// wait till the server is gracefully shutdown by using a WaitGroup in the Poison call.
	wg := &sync.WaitGroup{}
	e.Poison(serverPID, wg)
	wg.Wait()

}
