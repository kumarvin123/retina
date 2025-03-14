#include <winsock2.h>
#include <iphlpapi.h>
#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include <vector>
#include "event_writer.h"
#include <ebpf_api.h>

bpf_object *obj = NULL;
bpf_link* link = NULL;

int
set_filter(struct filter* flt) {
    uint8_t key = 0;
    int map_flt_fd = 0;

    // Attempt to open the pinned map
    map_flt_fd = bpf_obj_get(FILTER_MAP_PIN_PATH);
    if (map_flt_fd < 0) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        return 1;
    }
    if (bpf_map_update_elem(map_flt_fd, &key, flt, 0) != 0) {
        fprintf(stderr, "%s - failed to update filter\n", __FUNCTION__);
        return 1;
    }
    return 0;
}

int _pin(const char* pin_path, int fd, bool is_map) {
    int pin_fd = bpf_obj_get(pin_path);
    if (pin_fd < 0) {
        if (bpf_obj_pin(fd, pin_path) < 0) {
            fprintf(stderr, "%s - failed to pin %s to %s\n", __FUNCTION__,
                    is_map ? "map" : "prog", pin_path);
            return 1;
        }

        printf("%s - %s successfully pinned at %s\n", __FUNCTION__,
                    is_map ? "map" : "prog",
                    pin_path);
    } else {
        printf("%s - pinned %s found %s\n", __FUNCTION__,
                                            is_map ? "map" : "prog",
                                            pin_path);
    }
    return 0;
}

int
attach_program_to_interface(int ifindx) {
    printf("%s - Attaching program to interface with ifindex %d\n", __FUNCTION__, ifindx);
    struct bpf_program* prg = bpf_object__find_program_by_name(obj, "event_writer");

    if (prg == NULL) {
        fprintf(stderr, "%s - failed to find event_writer program", __FUNCTION__);
        return 1;
    }

    link = bpf_program__attach_xdp(prg, ifindx);
    if (link == NULL) {
        fprintf(stderr, "%s - failed to attach to interface with ifindex %d\n", __FUNCTION__, ifindx);
        return 1;
    }

    return 0;
}

int
load_and_pin(void) {
    struct bpf_program* prg = NULL;
    struct bpf_map *map_ev = NULL, *map_met = NULL, *map_fvt = NULL, *map_flt = NULL;
    int prg_fd = 0;

    // Load the BPF object file
    obj = bpf_object__open("bpf_event_writer.sys");
    if (obj == NULL) {
        fprintf(stderr, "%s - failed to open BPF object\n", __FUNCTION__);
        goto cleanup;
    }

    // Load cilium_events map and event_writer bpf program
    if (bpf_object__load(obj) < 0) {
        fprintf(stderr, "%s - failed to load BPF sys\n", __FUNCTION__);
        goto cleanup;
    }

    // Find the map by its name
    map_ev = bpf_object__find_map_by_name(obj, "cilium_events");
    if (map_ev == NULL) {
        fprintf(stderr, "%s - failed to find cilium_events by name\n", __FUNCTION__);
        goto cleanup;
    }
    if (_pin(EVENTS_MAP_PIN_PATH, bpf_map__fd(map_ev), true) != 0) {
        goto cleanup;
    }

    // Find the map by its name
    map_met = bpf_object__find_map_by_name(obj, "cilium_metrics");
    if (map_met == NULL) {
        fprintf(stderr, "%s - failed to find cilium_metrics by name\n", __FUNCTION__);
        goto cleanup;
    }
    if (_pin(METRICS_MAP_PIN_PATH, bpf_map__fd(map_ev), true) != 0) {
        goto cleanup;
    }

    // Find the map by its name
    map_fvt = bpf_object__find_map_by_name(obj, "five_tuple_map");
    if (map_fvt == NULL) {
        fprintf(stderr, "%s - failed to find five_tuple_map by name\n", __FUNCTION__);
        goto cleanup;
    }
    if (_pin(FIVE_TUPLE_MAP_PIN_PATH, bpf_map__fd(map_fvt), true) != 0) {
        goto cleanup;
    }

    // Find the map by its name
    map_flt = bpf_object__find_map_by_name(obj, "filter_map");
    if (map_flt == NULL) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        goto cleanup;
    }
    if (_pin(FILTER_MAP_PIN_PATH, bpf_map__fd(map_flt), true) != 0) {
        goto cleanup;
    }

    return 0; // Return success

cleanup:
    if (prg != NULL) {
        bpf_program__unpin(prg, EVENT_WRITER_PIN_PATH);
    }

    if (map_ev != NULL) {
        bpf_map__unpin(map_ev, EVENTS_MAP_PIN_PATH);
    }

    if (map_flt != NULL) {
        bpf_map__unpin(map_flt, FILTER_MAP_PIN_PATH);
    }

    if (map_fvt != NULL) {
        bpf_map__unpin(map_fvt, FIVE_TUPLE_MAP_PIN_PATH);
    }

    if (map_met != NULL) {
        bpf_map__unpin(map_met, METRICS_MAP_PIN_PATH);
    }

    if (obj != NULL) {
        bpf_object__close(obj);
    }
    return 1;
}

// Function to unload programs and detach
int
unload_programs_detach() {
    int fd = 0;
    int link_fd = 0;

    if (bpf_object__unpin_maps(obj, EVENTS_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "Failed to unpin BPF program");
        return 1;
    }
    if (bpf_object__unpin_maps(obj, METRICS_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "Failed to unpin BPF program");
        return 1;
    }
    if (bpf_object__unpin_maps(obj, FIVE_TUPLE_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "Failed to unpin BPF program");
        return 1;
    }

    link_fd = bpf_link__fd(link);
    if (bpf_link_detach(link_fd) != 0) {
        fprintf(stderr, "%s - failed to detach link\n", __FUNCTION__);
    }
    if (bpf_link__destroy(link) != 0) {
        fprintf(stderr, "%s - failed to destroy link", __FUNCTION__);
    }

    if (obj != NULL) {
        bpf_object__close(obj);
    }

    printf("%s - unloaded successfully\n", __FUNCTION__);
    return 0;
}

uint32_t _ipStrToUint(const char* ipStr) {
    uint32_t ip = 0;
    int part = 0;
    int parts = 0;
    const char *p = ipStr;
    char c;

    while ((c = *p++) != '\0') {
        if (c >= '0' && c <= '9') {
            part = part * 10 + (c - '0');
        } else if (c == '.') {
            ip = (ip << 8) | (part & 0xFF);
            part = 0;
            parts++;
        } else {
            // Invalid character in IP string.
            return 0;
        }
    }

    // Process the last octet.
    ip = (ip << 8) | (part & 0xFF);
    parts++;

    // Ensure we have exactly four parts
    if (parts != 4) {
        return 0;
    }

    return ip;
}

int main(int argc, char* argv[]) {
    setvbuf(stdout, NULL, _IONBF, 0);
    // Parse the command-line arguments (flags)
    if (argc < 2) {
        fprintf(stderr, "valid arguments are required. Exiting..\n");
        return 1;
    }

    if (strcmp(argv[1], "-pinmaps") == 0) {
        if (load_and_pin() != 0) {
            return 1;
        }
    } else if (strcmp(argv[1], "-start") == 0) {
        struct filter flt;
        int ifindx = 0;
        memset(&flt, 0, sizeof(flt));

        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-event") == 0) {
                if (i + 1 < argc)
                    flt.event = static_cast<uint8_t>(atoi(argv[++i]));
            } else if (strcmp(argv[i], "-srcIP") == 0) {
                if (i + 1 < argc)
                    flt.srcIP = _ipStrToUint(argv[++i]);
            } else if (strcmp(argv[i], "-dstIP") == 0) {
                if (i + 1 < argc)
                    flt.dstIP = _ipStrToUint(argv[++i]);
            } else if (strcmp(argv[i], "-srcprt") == 0) {
                if (i + 1 < argc)
                    flt.srcprt = static_cast<uint16_t>(atoi(argv[++i]));
            } else if (strcmp(argv[i], "-dstprt") == 0) {
                if (i + 1 < argc)
                    flt.dstprt = static_cast<uint16_t>(atoi(argv[++i]));
            } else if (strcmp(argv[i], "-ifindx") == 0) {
                if (i + 1 < argc)
                    ifindx = static_cast<uint16_t>(atoi(argv[++i]));
            }
        }
        printf("Parsed Values:\n");
        printf("Event: %d\n", flt.event);
        printf("Source IP: %u.%u.%u.%u\n",
               (flt.srcIP >> 24) & 0xFF, (flt.srcIP >> 16) & 0xFF,
               (flt.srcIP >> 8) & 0xFF, flt.srcIP & 0xFF);
        printf("Destination IP: %u.%u.%u.%u\n",
               (flt.dstIP >> 24) & 0xFF, (flt.dstIP >> 16) & 0xFF,
               (flt.dstIP >> 8) & 0xFF, flt.dstIP & 0xFF);
        printf("Source Port: %u\n", flt.srcprt);
        printf("Destination Port: %u\n", flt.dstprt);
        printf("Interface Index: %d\n", ifindx);

        if (set_filter(&flt) != 0) {
            return 1;
        } else {
            printf("%s - filter updated successfully\n", __FUNCTION__);
        }

        if (ifindx <= 0) {
            fprintf(stderr, "valid ifindx is required. Exiting..\n");
            return 1;
        }

        if (attach_program_to_interface(ifindx) != 0) {
            return 1;
        }

        //Sleep for 1 minute
        printf("%s - holding for 1 minute!!\n", __FUNCTION__);
        Sleep(60000);
        unload_programs_detach();
    } else {
        fprintf(stderr, "invalid arguments. Exiting..\n");
        return 1;
    }

    return 0;
}