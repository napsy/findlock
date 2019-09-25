# findlock
Parses Go tracebacks and finds possible deadlocks

This works by checking how many locks have the same memory address. If there are multiple goroutines with the same lock but have the same stack trace, then only one entry is shown. The tool is not perfect but helped me to find locks where I would have over 10k goroutines.
