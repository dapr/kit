# Cron

This package is a fork of [`github.com/robfig/cron/v3@v3.0.0`](https://github.com/robfig/cron/) which adds support for mocking the time using [`k8s.io/utils/clock`](https://k8s.io/utils/clock)

See [LICENSE](./LICENSE) for the license of the original package.

<!--
TODO : Remove this package if this PR gets merged https://github.com/robfig/cron/pull/327
-->

## Using Cron with Mock Clock

```go
import "k8s.io/utils/clock"

clk := clock.RealClock{}
c := cron.New(cron.WithClock(clk))
c.AddFunc("@every 1h", func() {
 fmt.Println("Every hour")
})
c.Start()
clk.Add(1 * time.Hour)
```
