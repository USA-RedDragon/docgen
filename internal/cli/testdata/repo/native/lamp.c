#define LAMP_MAX_TRIES 3

static int tries;

int lamp_start(int count)
{
	if (count > LAMP_MAX_TRIES) {
		return -1;
	}
	tries = count;
	return 0;
}
