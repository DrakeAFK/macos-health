//go:build darwin && cgo

// Read-only IOReport and AppleSMC protocol implementation. ABI and channel/key
// conventions reference macmon (MIT); see THIRD_PARTY_NOTICES.md.
#include "silicon_native.h"
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <dlfcn.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

typedef struct { uint32_t size, type; uint8_t attr; } key_info;
typedef struct {
 uint32_t key;
 struct { uint8_t major, minor, build, reserved; uint16_t release; } vers;
 struct { uint16_t version,length; uint32_t cpu,gpu,mem; } limits;
 key_info info; uint8_t result,status,cmd; uint32_t index; uint8_t bytes[32];
} key_data;
typedef struct {uint32_t key; key_info info; int kind;} sensor_key;
struct mh_native {
 void *lib; CFTypeRef sub; CFMutableDictionaryRef channels, subscribed;
 CFDictionaryRef previous;
 io_connect_t smc;
 sensor_key keys[128]; int key_count;
 CFDictionaryRef (*all)(uint64_t,uint64_t);
 CFTypeRef (*subscribe)(void*,CFMutableDictionaryRef,CFMutableDictionaryRef*,uint64_t,CFTypeRef);
 CFDictionaryRef (*sample)(CFTypeRef,CFMutableDictionaryRef,CFTypeRef);
 CFDictionaryRef (*delta)(CFDictionaryRef,CFDictionaryRef,CFTypeRef);
 CFStringRef (*group)(CFDictionaryRef), (*subgroup)(CFDictionaryRef), (*name)(CFDictionaryRef), (*unit)(CFDictionaryRef);
 int64_t (*integer)(CFDictionaryRef,int);
 int (*state_count)(CFDictionaryRef);
 CFStringRef (*state_name)(CFDictionaryRef,int);
 int64_t (*residency)(CFDictionaryRef,int);
};
static void string(CFStringRef s,char *out,size_t n) {out[0]=0;if(s && CFGetTypeID(s)==CFStringGetTypeID()) CFStringGetCString(s,out,n,kCFStringEncodingUTF8);}
static CFArrayRef array(CFDictionaryRef d) {if(!d||CFGetTypeID(d)!=CFDictionaryGetTypeID())return NULL;CFTypeRef a=CFDictionaryGetValue(d,CFSTR("IOReportChannels"));return a&&CFGetTypeID(a)==CFArrayGetTypeID()?(CFArrayRef)a:NULL;}
static uint32_t four(const char *s) {return ((uint32_t)(uint8_t)s[0]<<24)|((uint32_t)(uint8_t)s[1]<<16)|((uint32_t)(uint8_t)s[2]<<8)|(uint8_t)s[3];}
static int smc_call(io_connect_t c,key_data *in,key_data *out) {size_t n=sizeof(*out);memset(out,0,n);return c && IOConnectCallStructMethod(c,2,in,sizeof(*in),out,&n)==KERN_SUCCESS && n==sizeof(*out) && out->result==0;}
static int key_meta(io_connect_t c,uint32_t key,key_info *info) {key_data in={0},out={0};in.key=key;in.cmd=9;if(!smc_call(c,&in,&out)||out.info.size>32)return 0;*info=out.info;return 1;}
static int key_read(io_connect_t c,uint32_t key,key_info info,key_data *out) {key_data in={0};in.key=key;in.info=info;in.cmd=5;return smc_call(c,&in,out);}
static double number(key_data *d,key_info info) {
 if(info.type==four("flt ")&&info.size==4){float f;memcpy(&f,d->bytes,4);return f;}
 if(info.type==four("fpe2")&&info.size==2)return ((d->bytes[0]<<8)|d->bytes[1])/4.0;
 return NAN;
}
static void smc_init(mh_native *h) {
 io_iterator_t it=0;
 if(IOServiceGetMatchingServices(0,IOServiceMatching("AppleSMC"),&it)!=KERN_SUCCESS)return;
 io_service_t dev;
 while((dev=IOIteratorNext(it))){io_name_t name={0};IORegistryEntryGetName(dev,name);if(!strcmp(name,"AppleSMCKeysEndpoint"))IOServiceOpen(dev,mach_task_self(),0,&h->smc);IOObjectRelease(dev);if(h->smc)break;}
 IOObjectRelease(it);if(!h->smc)return;
 key_info info;key_data value={0};
 if(!key_meta(h->smc,four("#KEY"),&info)||info.size!=4||!key_read(h->smc,four("#KEY"),info,&value))return;
 uint32_t count=((uint32_t)value.bytes[0]<<24)|((uint32_t)value.bytes[1]<<16)|((uint32_t)value.bytes[2]<<8)|value.bytes[3];
 if(count>8192)return;
 for(uint32_t i=0;i<count && h->key_count<128;i++) {
  key_data in={0},out={0};in.cmd=8;in.index=i;if(!smc_call(h->smc,&in,&out))continue;
  char k[5]={(char)(out.key>>24),(char)(out.key>>16),(char)(out.key>>8),(char)out.key,0};int kind=0;
  if(k[0]=='T'&&(k[1]=='p'||k[1]=='e'||k[1]=='s'))kind=1;
  else if(k[0]=='T'&&k[1]=='g')kind=2;
  else if(k[0]=='F'&&k[2]=='A'&&k[3]=='c')kind=3;
  if(kind && key_meta(h->smc,out.key,&info) && (info.type==four("flt ") || (kind==3 && info.type==four("fpe2"))))h->keys[h->key_count++]=(sensor_key){out.key,info,kind};
 }
}
#define LOAD(field,symbol) h->field=dlsym(h->lib,symbol);if(!h->field)goto no_report
mh_native *mh_open(void) {
 mh_native *h=calloc(1,sizeof(*h));if(!h)return NULL;
 smc_init(h);
 h->lib=dlopen("/usr/lib/libIOReport.dylib",RTLD_LAZY|RTLD_LOCAL);if(!h->lib)return h;
 LOAD(all,"IOReportCopyAllChannels");LOAD(subscribe,"IOReportCreateSubscription");LOAD(sample,"IOReportCreateSamples");LOAD(delta,"IOReportCreateSamplesDelta");LOAD(group,"IOReportChannelGetGroup");LOAD(subgroup,"IOReportChannelGetSubGroup");LOAD(name,"IOReportChannelGetChannelName");LOAD(unit,"IOReportChannelGetUnitLabel");LOAD(integer,"IOReportSimpleGetIntegerValue");LOAD(state_count,"IOReportStateGetCount");LOAD(state_name,"IOReportStateGetNameForIndex");LOAD(residency,"IOReportStateGetResidency");
 CFDictionaryRef all=h->all(0,0);CFArrayRef entries=array(all);
 if(entries) {
  h->channels=CFDictionaryCreateMutableCopy(NULL,0,all);
  CFMutableArrayRef selected=CFArrayCreateMutable(NULL,0,&kCFTypeArrayCallBacks);
  for(CFIndex i=0;i<CFArrayGetCount(entries);i++){
   CFDictionaryRef item=CFArrayGetValueAtIndex(entries,i);if(CFGetTypeID(item)!=CFDictionaryGetTypeID())continue;
   char group[128],subgroup[128];string(h->group(item),group,sizeof(group));string(h->subgroup(item),subgroup,sizeof(subgroup));
   if(!strcmp(group,"Energy Model")||!strcmp(subgroup,"GPU Performance States"))CFArrayAppendValue(selected,item);
  }
  CFDictionarySetValue(h->channels,CFSTR("IOReportChannels"),selected);CFRelease(selected);
  h->sub=h->subscribe(NULL,h->channels,&h->subscribed,0,NULL);
 }
 if(all)CFRelease(all);
 return h;
no_report:
 dlclose(h->lib);h->lib=NULL;return h;
}
mh_reading mh_sample(mh_native *h,double dt) {
 mh_reading r={0};if(!h)return r;
 double csum=0,gsum=0;int cn=0,gn=0;
 for(int i=0;i<h->key_count;i++){
  sensor_key key=h->keys[i];key_data v={0};if(!key_read(h->smc,key.key,key.info,&v))continue;
  double n=number(&v,key.info);if(!isfinite(n))continue;
  if(key.kind==3){if(n>=0&&n<20000&&r.fan_count<8)r.fans[r.fan_count++]=n;}
  else if(n>0&&n<150){if(key.kind==1){csum+=n;cn++;}else{gsum+=n;gn++;}}
 }
 if(cn){r.cpu_t=csum/cn;r.flags|=16;}if(gn){r.gpu_t=gsum/gn;r.flags|=32;}
 if(!h->sub)return r;
 CFDictionaryRef next=h->sample(h->sub,h->channels,NULL);if(!next)return r;
 CFDictionaryRef diff=NULL;if(h->previous&&dt>0&&dt<30)diff=h->delta(h->previous,next,NULL);
 if(h->previous)CFRelease(h->previous);h->previous=next;
 CFArrayRef entries=array(diff);double gpu_total=0,gpu_active=0;
 if(entries)for(CFIndex i=0;i<CFArrayGetCount(entries);i++){
  CFDictionaryRef item=CFArrayGetValueAtIndex(entries,i);if(CFGetTypeID(item)!=CFDictionaryGetTypeID())continue;
  char group[128],subgroup[128],name[128],unit[32];string(h->group(item),group,sizeof(group));string(h->subgroup(item),subgroup,sizeof(subgroup));string(h->name(item),name,sizeof(name));string(h->unit(item),unit,sizeof(unit));
  if(!strcmp(group,"Energy Model")){
   double scale=!strcmp(unit,"mJ")?1e3:!strcmp(unit,"uJ")?1e6:!strcmp(unit,"nJ")?1e9:0;
   int64_t raw=h->integer(item,0);if(!scale||raw<0)continue;double w=(double)raw/scale/dt;if(!isfinite(w)||w>2000)continue;
   size_t len=strlen(name);
   if(len>=10&&!strcmp(name+len-10,"CPU Energy")){r.cpu_w+=w;r.flags|=1;}
   else if(len>=10&&!strcmp(name+len-10,"GPU Energy")){r.gpu_w+=w;r.flags|=2;}
   else if(!strncmp(name,"ANE",3)){r.ane_w+=w;r.flags|=4;}
  }else if(!strcmp(subgroup,"GPU Performance States")){
   int count=h->state_count(item);if(count<1||count>256)continue;
   for(int j=0;j<count;j++){char state[64];string(h->state_name(item,j),state,sizeof(state));int64_t n=h->residency(item,j);if(n<0)continue;gpu_total+=n;if(strcmp(state,"OFF")&&strcmp(state,"IDLE")&&strcmp(state,"DOWN"))gpu_active+=n;}
  }
 }
 if(gpu_total>0){r.gpu_pct=100*gpu_active/gpu_total;r.flags|=8;}
 if(diff)CFRelease(diff);return r;
}
void mh_close(mh_native *h) {
 if(!h)return;if(h->previous)CFRelease(h->previous);if(h->sub)CFRelease(h->sub);if(h->subscribed)CFRelease(h->subscribed);if(h->channels)CFRelease(h->channels);if(h->smc)IOServiceClose(h->smc);if(h->lib)dlclose(h->lib);free(h);
}
