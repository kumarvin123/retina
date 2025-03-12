#include <winsock2.h>
#include <iphlpapi.h>
#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include <vector>
#include "event_writer.h"
#include <vector>

std::vector<std::pair<int, struct bpf_link*>> link_list;
bpf_object* obj = NULL;

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

int pin_map(const char* pin_path, bpf_map* map) {
    int map_fd = 0;
    // Attempt to open the pinned map
    map_fd = bpf_obj_get(pin_path);
    if (map_fd < 0) {
        // Get the file descriptor of the map
        map_fd = bpf_map__fd(map);

        if (map_fd < 0) {
            fprintf(stderr, "%s - failed to get map file descriptor\n", __FUNCTION__);
            return 1;
        }

        if (bpf_obj_pin(map_fd, pin_path) < 0) {
            fprintf(stderr, "%s - failed to pin map to %s\n", __FUNCTION__, pin_path);
            return 1;
        }

        printf("%s - map successfully pinned at %s\n", __FUNCTION__, pin_path);
    } else {
        printf("%s -pinned map found %s\n", __FUNCTION__, pin_path);
    }
    return 0;
}

int get_physical_interface_indice() {
    // Command to extract the first Mellanox adapter ifIndex using PowerShell.
    const char* cmd = "powershell -Command \"Get-NetAdapter | Where-Object { $_.InterfaceDescription -match 'Mellanox' } | Select-Object -First 1 | ForEach-Object { Write-Output $_.ifIndex }\"";

    // Open a pipe to execute the command.
    FILE* pipe = _popen(cmd, "r");
    if (!pipe) {
        fprintf(stderr, "Failed to run command\n");
        return -1;
    }

    char buffer[128];
    memset(buffer, '\0', sizeof(buffer));
    std::string result;
    // Read the output of the command.
    while (fgets(buffer, sizeof(buffer), pipe) != nullptr) {
        result += buffer;
    }
    _pclose(pipe);

    // Check if result is empty.
    if (result.empty()) {
        fprintf(stderr, "No output received from PowerShell command; cannot extract ifIndex.\n");
        return -1;
    }

    // Convert the output string to an integer.
    int ifIndex = atoi(result.c_str());
    printf("Extracted ifIndex: %d\n", ifIndex);
    return ifIndex;
}

int
attach_program_to_interface(int ifindx) {
    printf("%s - Attaching program to interface with ifindex %d\n", __FUNCTION__, ifindx);
    struct bpf_program* prg = bpf_object__find_program_by_name(obj, "event_writer");
    bpf_link* link = NULL;
    if (prg == NULL) {
        fprintf(stderr, "%s - failed to find event_writer program", __FUNCTION__);
        return 1;
    }

    link = bpf_program__attach_xdp(prg, ifindx);
    if (link == NULL) {
        fprintf(stderr, "%s - failed to attach to interface with ifindex %d\n", __FUNCTION__, ifindx);
        return 1;
    }

    link_list.push_back(std::pair<int, bpf_link*>{ifindx, link});
    return 0;
}

int
pin_maps_load_programs(void) {
    struct bpf_program* prg = NULL;
    struct bpf_map *map_ev, *map_met, *map_fvt, *map_flt;

    // Load the BPF object file
    obj = bpf_object__open("bpf_event_writer.sys");
    if (obj == NULL) {
        fprintf(stderr, "%s - failed to open BPF object\n", __FUNCTION__);
        return 1;
    }

    // Load cilium_events map and event_writer bpf program
    if (bpf_object__load(obj) < 0) {
        fprintf(stderr, "%s - failed to load BPF sys\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }

    // Find the map by its name
    map_ev = bpf_object__find_map_by_name(obj, "cilium_events");
    if (map_ev == NULL) {
        fprintf(stderr, "%s - failed to find cilium_events by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(EVENTS_MAP_PIN_PATH, map_ev) != 0) {
        return 1;
    }

    // Find the map by its name
    map_met = bpf_object__find_map_by_name(obj, "cilium_metrics");
    if (map_met == NULL) {
        fprintf(stderr, "%s - failed to find cilium_metrics by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(METRICS_MAP_PIN_PATH, map_ev) != 0) {
        return 1;
    }

    // Find the map by its name
    map_fvt = bpf_object__find_map_by_name(obj, "five_tuple_map");
    if (map_fvt == NULL) {
        fprintf(stderr, "%s - failed to find five_tuple_map by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(FIVE_TUPLE_MAP_PIN_PATH, map_fvt) != 0) {
        return 1;
    }

    // Find the map by its name
    map_flt = bpf_object__find_map_by_name(obj, "filter_map");
    if (map_flt == NULL) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        return 1;
    }
    if (pin_map(FILTER_MAP_PIN_PATH, map_flt) != 0) {
        return 1;
    }

    return 0; // Return success
}

// Function to unload programs and detach
int
unload_programs_detach() {

    for (auto it = link_list.begin(); it != link_list.end(); it ++) {
        auto ifidx = it->first;
        auto link = it->second;
        auto link_fd = bpf_link__fd(link);
        if (bpf_link_detach(link_fd) != 0) {
            fprintf(stderr, "%s - failed to detach link %d\n", __FUNCTION__, ifidx);
        }
        if (bpf_link__destroy(link) != 0) {
            fprintf(stderr, "%s - failed to destroy link %d", __FUNCTION__, ifidx);
        }
    }

    if (obj != NULL) {
        bpf_object__close(obj);
    }

    printf("%s - unloaded successfully\n", __FUNCTION__);
    return 0;
}

uint32_t ipStrToUint(const char* ipStr) {
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
    struct filter flt;
    setvbuf(stdout, NULL, _IONBF, 0);
    memset(&flt, 0, sizeof(flt));
    // Parse the command-line arguments (flags)
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-pinmaps") == 0) {
            return pin_maps_load_programs();
        }
        else if (strcmp(argv[i], "-event") == 0) {
            if (i + 1 < argc)
                flt.event = static_cast<uint8_t>(atoi(argv[++i]));
        } else if (strcmp(argv[i], "-srcIP") == 0) {
            if (i + 1 < argc)
                flt.srcIP = ipStrToUint(argv[++i]);
        } else if (strcmp(argv[i], "-dstIP") == 0) {
            if (i + 1 < argc)
                flt.dstIP = ipStrToUint(argv[++i]);
        } else if (strcmp(argv[i], "-srcprt") == 0) {
            if (i + 1 < argc)
                flt.srcprt = static_cast<uint16_t>(atoi(argv[++i]));
        } else if (strcmp(argv[i], "-dstprt") == 0) {
            if (i + 1 < argc)
                flt.dstprt = static_cast<uint16_t>(atoi(argv[++i]));
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
    printf("Starting event writer\n");

    if (pin_maps_load_programs() != 0) {
        return 1;
    }

    if (set_filter(&flt) != 0) {
        return 1;
    } else {
        printf("%s - filter updated successfully\n", __FUNCTION__);
    }

    int ifindx = get_physical_interface_indice();
    if (ifindx == -1) {
        return 1;
    }

    if (attach_program_to_interface(ifindx) != 0) {
        return 1;
    }

    //Sleep for 1 minute
    printf("%s - holding for 1 minute!!\n", __FUNCTION__);
    Sleep(60000);
    unload_programs_detach();
    return 0;
}