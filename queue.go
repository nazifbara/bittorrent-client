package main

import (
	"fmt"
)

func pendingKey(index, begin uint32) string {
	return fmt.Sprintf("%d:%d", index, begin)
}

func (c *Client) enqueue(job *Job) {
	select {
	case c.queue <- job:
	case <-c.ctx.Done():
	}
}

func (c *Client) requeue(job *Job) {
	select {
	case c.queue <- job:
	case <-c.ctx.Done():
	default:
		go func() {
			select {
			case c.queue <- job:
			case <-c.ctx.Done():
			}
		}()
	}
}

func (c *Client) onTimeout(key string, job *Job) {
	c.mu.Lock()
	_, stillPending := c.pending[key]
	delete(c.pending, key)
	c.mu.Unlock()

	if !stillPending {
		return
	}

	c.requeue(job)
}

func (c *Client) shutdown() {
	c.cancel()
	c.wg.Wait()
}

func (c *Client) addPieceJobs(pieceIdx uint32) {
	pieceState := c.piecesGrid[pieceIdx]
	pieceState.mu.Lock()
	total := pieceState.NumOfBlocks
	pieceState.mu.Unlock()
	for i := range total {
		offset := i * int(blockSize)
		c.AddToQueue(pieceIdx, uint32(offset))
	}
}

func (c *Client) AddToQueue(pieceIndex uint32, offset uint32) {
	pieceState := c.piecesGrid[pieceIndex]
	pieceState.mu.Lock()
	if pieceState.Done {
		pieceState.mu.Unlock()
		return
	}
	pieceState.mu.Unlock()
	c.enqueue(&Job{index: pieceIndex, begin: offset})
}
