#ifndef MH_SILICON_H
#define MH_SILICON_H
#include <stdint.h>
typedef struct mh_native mh_native;
typedef struct { double cpu_w, gpu_w, ane_w, gpu_pct, cpu_t, gpu_t; unsigned flags; double fans[8]; int fan_count; } mh_reading;
mh_native *mh_open(void);
mh_reading mh_sample(mh_native *, double);
void mh_close(mh_native *);
#endif
