#include <time.h>

void sleep_ms(long long nanoseconds) {
    struct timespec ts;
    ts.tv_sec = nanoseconds / 1000000000;
    ts.tv_nsec = nanoseconds % 1000000000;
    nanosleep(&ts, NULL);
}
