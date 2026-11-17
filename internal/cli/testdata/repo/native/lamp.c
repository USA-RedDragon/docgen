#define LAMP_MAX_TRIES 3

static int tries;

struct lamp_config {
	int max_tries;
	int (*on_fail)(int count);
};

typedef struct lamp_config lamp_config_t;

int lamp_start(int count)
{
	if (count > LAMP_MAX_TRIES) {
		return -1;
	}
	tries = count;
	return 0;
}
