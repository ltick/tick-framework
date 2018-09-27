package pooling

type Conn interface {
	BindPool(pool Pool) Conn
	Close() error
	Do(doFn func(interface{}) error) error
	Recycle()
}

type conn struct {
	idle    interface{}
	closeFn func() error
	pool    Pool
	err     error
}

func NewConn(idle interface{}, closeFn func() error) Conn {
	return &conn{
		idle:    idle,
		closeFn: closeFn,
	}
}

func (c *conn) BindPool(pool Pool) Conn {
	c.pool = pool
	return c
}

func (c *conn) Close() error {
	return c.closeFn()
}

func (c *conn) Do(doFn func(interface{}) error) error {
	c.err = doFn(c.idle)
	return c.err
}

func (c *conn) Recycle() {
	if c.err != nil {
		c.pool.Remove(c)
	} else {
		c.pool.Put(c)
	}
}
