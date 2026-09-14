//go:build darwin && cgo

package stats

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <sys/resource.h>
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <mach/vm_page_size.h>
#include <stdlib.h>
#include <stdint.h>
#include <sys/sysctl.h>
#include <net/if.h>
#include <net/route.h>
#include <string.h>

typedef struct {
 int pid, parent; unsigned flags;
 uint64_t user_ns, system_ns, rss, footprint, read_bytes, write_bytes, start_sec, start_usec;
 char name[33];
} mh_proc;
static mh_proc *mh_processes(int *length) {
 *length=0;int n=proc_listallpids(NULL,0);if(n<=0||n>65536)return NULL;n+=64;
 int *pids=calloc(n,sizeof(int));mh_proc *rows=calloc(n,sizeof(mh_proc));if(!pids||!rows){free(pids);free(rows);return NULL;}
 int found=proc_listallpids(pids,n*sizeof(int));if(found>n)found=n;
 for(int i=0;i<found;i++) {
  if(pids[i]<=0)continue;struct proc_taskallinfo task={0};struct proc_bsdinfo info={0};struct rusage_info_v2 usage={0};
  int has_task=proc_pidinfo(pids[i],PROC_PIDTASKALLINFO,0,&task,sizeof(task))==sizeof(task);
  if(has_task)info=task.pbsd;else if(proc_pidinfo(pids[i],PROC_PIDTBSDINFO,0,&info,sizeof(info))!=sizeof(info))continue;
  int has_usage=proc_pid_rusage(pids[i],RUSAGE_INFO_V2,(rusage_info_t*)&usage)==0;
  mh_proc *r=&rows[(*length)++];r->pid=pids[i];r->parent=info.pbi_ppid;
  if(has_task){r->flags|=1;r->user_ns=task.ptinfo.pti_total_user;r->system_ns=task.ptinfo.pti_total_system;r->rss=task.ptinfo.pti_resident_size;}
  if(has_usage){r->flags|=3;r->user_ns=usage.ri_user_time;r->system_ns=usage.ri_system_time;r->rss=usage.ri_resident_size;r->footprint=usage.ri_phys_footprint;r->read_bytes=usage.ri_diskio_bytesread;r->write_bytes=usage.ri_diskio_byteswritten;}
  r->start_sec=info.pbi_start_tvsec;r->start_usec=info.pbi_start_tvusec;
  memcpy(r->name,info.pbi_name,sizeof(info.pbi_name));if(!r->name[0])memcpy(r->name,info.pbi_comm,sizeof(info.pbi_comm));
 }
 free(pids);return rows;
}
typedef struct {char name[IFNAMSIZ];uint64_t received,sent;} mh_net;
static mh_net *mh_network(int *count) {
 *count=0;int mib[6]={CTL_NET,PF_ROUTE,0,0,NET_RT_IFLIST2,0};size_t size=0;
 if(sysctl(mib,6,NULL,&size,NULL,0)!=0||size==0||size>(4<<20))return NULL;
 size+=4096;char *data=malloc(size);if(!data)return NULL;
 if(sysctl(mib,6,data,&size,NULL,0)!=0){free(data);return NULL;}
 mh_net *rows=calloc(size/sizeof(struct if_msghdr2)+1,sizeof(mh_net));if(!rows){free(data);return NULL;}
 for(size_t offset=0;offset+4<=size;){
  uint16_t length;memcpy(&length,data+offset,2);if(length<4||offset+length>size)break;
  if((unsigned char)data[offset+3]==RTM_IFINFO2&&length>=sizeof(struct if_msghdr2)){
   struct if_msghdr2 msg;memcpy(&msg,data+offset,sizeof(msg));mh_net *r=&rows[*count];
   if(if_indextoname(msg.ifm_index,r->name)){r->received=msg.ifm_data.ifi_ibytes;r->sent=msg.ifm_data.ifi_obytes;(*count)++;}
  }
  offset+=length;
 }
 free(data);return rows;
}
static int mh_vm(uint64_t *size,uint64_t *compressed,uint64_t *pageouts){
 vm_statistics64_data_t vm={0};mach_msg_type_number_t n=HOST_VM_INFO64_COUNT;mach_port_t host=mach_host_self();
 kern_return_t result=host_statistics64(host,HOST_VM_INFO64,(host_info64_t)&vm,&n);mach_port_deallocate(mach_task_self(),host);
 if(result!=KERN_SUCCESS)return 0;*size=vm_kernel_page_size;*compressed=vm.compressor_page_count;*pageouts=vm.pageouts;return 1;
}
*/
import "C"

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	gnet "github.com/shirou/gopsutil/v3/net"
	"golang.org/x/sys/unix"
)

func nativeVMCounters() (vmCounters, error) {
	var size, compressed, pageouts C.uint64_t
	if C.mh_vm(&size, &compressed, &pageouts) == 0 {
		return vmCounters{}, fmt.Errorf("host_statistics64 unavailable")
	}
	return vmCounters{PageSize: uint64(size), CompressedPages: uint64(compressed), PageOuts: uint64(pageouts)}, nil
}

func (c *Collector) readProcesses(ctx context.Context, now time.Time) ([]parsedProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var n C.int
	ptr := C.mh_processes(&n)
	if ptr == nil {
		return nil, fmt.Errorf("libproc process list unavailable")
	}
	defer C.free(unsafe.Pointer(ptr))
	native := unsafe.Slice(ptr, int(n))
	rows := make([]parsedProcess, 0, len(native))
	total, _ := unix.SysctlUint64("hw.memsize")
	c.processMu.Lock()
	defer c.processMu.Unlock()
	for _, raw := range native {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		birth := time.Unix(int64(raw.start_sec), int64(raw.start_usec)*1000)
		pid := int32(raw.pid)
		name, app := "", ""
		if previous, ok := c.lastProcesses[pid]; ok && previous.startedAt.Equal(birth) {
			name, app = previous.name, previous.app
		}
		if name == "" {
			var path [4096]C.char
			if C.proc_pidpath(C.int(pid), unsafe.Pointer(&path[0]), C.uint32_t(len(path))) > 0 {
				command := C.GoString(&path[0])
				name = cleanText(filepath.Base(command))
				app = processApp(command)
			} else {
				name = cleanText(C.GoString(&raw.name[0]))
				app = name
			}
		}
		percent := 0.
		if total > 0 {
			percent = 100 * float64(raw.rss) / float64(total)
		}
		row := ProcessRow{MemoryAvailable: uint(raw.flags)&1 != 0, PID: pid, ParentPID: int32(raw.parent), Name: name, App: app, Kind: processKind(name), AgeSeconds: max(0, now.Sub(birth).Seconds()), StartedAt: birth, RSSBytes: uint64(raw.rss), FootprintBytes: uint64(raw.footprint), FootprintAvailable: uint(raw.flags)&2 != 0, MemoryPercent: percent}
		rows = append(rows, parsedProcess{row: row, startedAt: birth, cpuSeconds: (float64(raw.user_ns) + float64(raw.system_ns)) / 1e9, elapsed: row.AgeSeconds, readBytes: uint64(raw.read_bytes), writeBytes: uint64(raw.write_bytes), ioAvailable: uint(raw.flags)&2 != 0, cpuAvailable: uint(raw.flags)&1 != 0})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no accessible libproc processes")
	}
	return rows, nil
}

func nativeNetCounters(ctx context.Context) ([]gnet.IOCountersStat, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var count C.int
	ptr := C.mh_network(&count)
	if ptr == nil {
		return nil, fmt.Errorf("native interface counters unavailable")
	}
	defer C.free(unsafe.Pointer(ptr))
	result := make([]gnet.IOCountersStat, 0, int(count))
	for _, r := range unsafe.Slice(ptr, int(count)) {
		result = append(result, gnet.IOCountersStat{Name: cleanText(C.GoString(&r.name[0])), BytesRecv: uint64(r.received), BytesSent: uint64(r.sent)})
	}
	return result, nil
}
