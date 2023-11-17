package main

/*
#include <time.h>

void sleep(long long nanoseconds) {
    struct timespec ts;
    ts.tv_sec = nanoseconds / 1000000000;
    ts.tv_nsec = nanoseconds % 1000000000;
    nanosleep(&ts, NULL);
}
*/
import "C"
