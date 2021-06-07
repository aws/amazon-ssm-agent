# scheduler
[![GoDoc](https://godoc.org/github.com/carlescere/scheduler?status.svg)](https://godoc.org/github.com/carlescere/scheduler)
[![Build Status](https://travis-ci.org/carlescere/scheduler.svg?branch=master)](https://travis-ci.org/carlescere/scheduler)
[![Coverage Status](https://coveralls.io/repos/carlescere/scheduler/badge.svg?branch=master)](https://coveralls.io/r/carlescere/scheduler?branch=master)

Job scheduling made easy.

Scheduler allows you to schedule recurrent jobs with an easy-to-read syntax.

Inspired by the article **[Rethinking Cron](http://adam.heroku.com/past/2010/4/13/rethinking_cron/)** and the **[schedule](https://github.com/dbader/schedule)** python module.

## How to use?
```go
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/carlescere/scheduler"
)

func main() {
	job := func() {
		t := time.Now()
		fmt.Println("Time's up! @", t.UTC())
	}
      // Run every 2 seconds but not now.
	scheduler.Every(2).Seconds().NotImmediately().Run(job)
      
      // Run now and every X.
	scheduler.Every(5).Minutes().Run(job)
	scheduler.Every().Day().Run(job)
	scheduler.Every().Monday().At("08:30").Run(job)
      
      // Keep the program from not exiting.
	runtime.Goexit()
}
```

## How it works?
By specifying the chain of calls, a `Job` struct is instantiated and a goroutine is starts observing the `Job`.

The goroutine will be on pause until:
* The next run scheduled is due. This will cause to execute the job.
* The `SkipWait` channel is activated. This will cause to execute the job.
* The `Quit` channel is activated. This will cause to finish the job.

## Not immediate recurrent jobs
By default the behaviour of the recurrent jobs (Every(N) seconds, minutes, hours) is to start executing the job right away and then wait the required amount of time. By calling specifically `.NotImmediately()` you can override that behaviour and not execute it directly when the function `Run()` is called.

```go
scheduler.Every(5).Minutes().NotImmediately().Run(job)
```

## License
Distributed under MIT license. See `LICENSE` for more information.
