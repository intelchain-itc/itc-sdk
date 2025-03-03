#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
A python 3.6+ script to send a batch of transactions from a CSV file.

Note that this script must be in the same directory as the CLI binary.

Due to a possible Nonce mismatch, it is recommended to NOT have 1 'from' address/wallet
appear in multiple CSV files that are ran at the same time.

Example:
    ./itc-csv.py /path/to/csv/file.csv --node https://testnet.intelchain.network/
    ./itc-csv.py /path/to/csv/file.csv --fast -n https://testnet.intelchain.network/
    ./itc-csv.py /path/to/csv/file.csv --fast --use-default-passphrase --yes -n https://testnet.intelchain.network/
    ./itc-csv.py /path/to/csv/file.csv --fast --use-default-passphrase --yes --batch-size 100 -n https://testnet.intelchain.network/

Sample CSV file:
    https://docs.google.com/spreadsheets/d/1nkF8N16S3y28cn7SM1cYJca8lzHPOzyR42S1V-OOAeQ/edit?usp=sharing

For detail help message:
    ./itc-csv.py --help

Install with:
    curl -O https://raw.githubusercontent.com/intelchain-itc/itc-sdk/master/scripts/itc-csv.py && chmod +x itc-csv.py
"""
import sys
import time
import getpass
import argparse
import os
import csv
import subprocess
import urllib.request
import urllib.error
import json
import ssl

script_directory = os.path.dirname(os.path.realpath(__file__))
_itc_call_and_prefix = [f"{script_directory}/itc"]
chain_id_options = {"mainnet", "testnet", "stressnet", "partner", "dryrun"}
default_passphrase = ""


class Typgpy(str):
    """
    Typography constants for pretty printing.

    Note that an ENDC is needed to mark the end of a 'highlighted' text segment.
    """
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'


def _itc(cli_args, timeout=200):
    """
    Helper function to call the CLI with the given args.

    Assumes `_setup_itc` has been called prior to using this function.

    Raises subprocess.CalledProcessError if call errored.
    """
    assert isinstance(cli_args, list)
    itc_and_args = _itc_call_and_prefix + [str(x) for x in cli_args]
    if args.verbose:
        itc_and_args.append("--verbose")
    return subprocess.check_output(itc_and_args, env=os.environ, timeout=timeout).decode()


def get_shard_count(node):
    """
    Fetch the number of shards on the node.

    Will raise a KeyError if the RPC returns back an error.
    Will raise a subprocess.CalledProcessError if CLI errored.
    """
    response = _itc(["utility", "shards", "-n", node])
    return len(json.loads(response)['result'])


def send_transactions(transactions, batch_size, node, chain_id, timeout=40, fast=False, yes=False):
    """
    Send the `transactions` where each call to the CLI is `batch_size` transactions.
    If `fast` is enabled, the CLI will not wait for a transaction to confirm, otherwise it will
    wait at most `timeout` seconds to confirm transaction.

    One can bypass the input confirmation by enabling `yes`.
    """
    print(f"{Typgpy.HEADER}Transactions to send:{Typgpy.ENDC}")
    print(json.dumps(transactions, indent=2))
    print(f"{Typgpy.HEADER}Transaction Count: {Typgpy.OKGREEN}{len(transactions)}{Typgpy.ENDC}")
    print(f"{Typgpy.HEADER}Node/Endpoint: {Typgpy.OKGREEN}{node}{Typgpy.ENDC}")
    if chain_id:
        print(f"{Typgpy.HEADER}Chain-ID: {Typgpy.OKGREEN}{chain_id}{Typgpy.ENDC}")
    if not yes and input("Send Transactions? [Y]/n\n> ").lower() not in {"yes", "y"}:
        return

    for i in range(0, len(transactions), batch_size):
        batch_tx = transactions[i: i + batch_size]
        temp_file = f"/tmp/itc-csv-{hash(str(batch_tx))}.json"
        batch_log_file = f"{script_directory}/batch_tx_{time.time()}.log"
        with open(temp_file, "w") as f:
            json.dump(batch_tx, f)  # Assume to work since `transactions` should be built by `parse_csv`
        os.chmod(temp_file, 400)
        print(
            f"{Typgpy.OKBLUE}Sending a batch of {Typgpy.OKGREEN}{len(batch_tx)}{Typgpy.OKBLUE} transaction(s){Typgpy.ENDC}")
        print(f"{Typgpy.OKBLUE}Logs for this batch will be at {Typgpy.OKGREEN}{batch_log_file}{Typgpy.ENDC}")
        itc_args = ["transfer", "--file", temp_file, "--node", node]
        if chain_id:
            itc_args.extend(["--chain-id", chain_id])
        if fast:
            itc_args.extend(["--timeout", "0"])
        else:
            itc_args.extend(["--timeout", timeout])
        try:
            output = _itc(itc_args, timeout=timeout * len(batch_tx))
        except subprocess.CalledProcessError as e:
            print(f"{Typgpy.FAIL}Transaction failure: {e}{Typgpy.ENDC}")
            print(f"{Typgpy.FAIL}Error output: {e.output.decode()}{Typgpy.ENDC}")
            with open(batch_log_file, "w") as f:
                f.write(f"Sent-tx: {json.dumps(batch_tx, indent=2)}\nResponse: {e.output.decode()}")
            raise e
        finally:
            os.remove(temp_file)

        print(f"{Typgpy.OKGREEN}Batched transaction(s) sent successfully!{Typgpy.ENDC}")
        print(f"{Typgpy.OKGREEN}Transaction Hashes/Receipts: {Typgpy.ENDC}{output}")
        with open(batch_log_file, "w") as f:
            f.write(f"Sent-tx: {json.dumps(batch_tx, indent=2)}\nResponse: {output}")
    print(f"{Typgpy.BOLD}HOORAY! Sent all transactions!{Typgpy.ENDC}")


def parse_csv(path, node, use_default_passphrase=True):
    """
    Parses the CSV into a list of dicts, loosely resembling the batch transaction format.

    Assumes that the given path is a CSV file and that the file exists.
    Assumes that all given CSV fields are strings.
    """
    columns = {"from", "to", "amount", "from-shard", "to-shard", "passphrase-file", "passphrase-string", "gas-price",
               "gas-limit"}

    def row_filter(row):
        valid_row = False
        for col in columns:
            if col not in row:
                return False
            if row[col]:
                valid_row = True
        return valid_row

    data, known_from_passphrase = [], {}
    shard_count = get_shard_count(node)
    with open(path, 'r') as f:
        print(f"Parsing CSV at {Typgpy.OKGREEN}{path}{Typgpy.ENDC}")
        for i, row in enumerate(filter(row_filter, csv.DictReader(f))):
            sys.stdout.write(f"\rParsing line {i} of {path}")
            sys.stdout.flush()
            try:
                _itc(["utility", "bech32-to-addr", row['from']])
                _itc(["utility", "bech32-to-addr", row['to']])
            except subprocess.CalledProcessError as e:
                print(f"{e.output}")
                print(f"{Typgpy.FAIL}Address error on line {i}! From: {row['from']}; To: {row['to']}{Typgpy.ENDC}")
                print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                continue
            if not row['from-shard'] or not row['to-shard']:
                print(f"{Typgpy.FAIL}To and/or from shard is not provided on line {i}!{Typgpy.ENDC}")
                print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                continue
            try:
                if int(row['from-shard']) >= shard_count:
                    print(f"{Typgpy.FAIL}From shard ({row['from-shard']}) on line {i} "
                          f"is >= number of shards ({shard_count}){Typgpy.ENDC}")
                    print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                    continue
                if int(row['to-shard']) >= shard_count:
                    print(f"{Typgpy.FAIL}To shard ({row['to-shard']}) on line {i} "
                          f"is >= number of shards ({shard_count}){Typgpy.ENDC}")
                    print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                    continue
            except ValueError as e:
                print(f"{Typgpy.FAIL}Error on line {i}: {e}{Typgpy.ENDC}")
                print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                continue
            # Required fields should be valid at this point.
            txn = {
                "from": row["from"],
                "to": row["to"],
                "amount": row["amount"],
                "from-shard": row["from-shard"],
                "to-shard": row["to-shard"],
                "stop-on-error": True
            }
            if row['passphrase-file']:
                txn['passphrase-file'] = row['passphrase-file']
            elif row['passphrase-string']:
                txn['passphrase-string'] = row['passphrase-string']
            else:
                if use_default_passphrase:
                    known_from_passphrase[row["from"]] = default_passphrase
                elif row["from"] not in known_from_passphrase:
                    print()
                    prompt = f"Enter passphrase for wallet {Typgpy.OKGREEN}{row['from']}{Typgpy.ENDC}\n> "
                    known_from_passphrase[row["from"]] = getpass.getpass(prompt=prompt)
                txn['passphrase-string'] = known_from_passphrase[row["from"]]
            if row["gas-price"]:
                try:
                    int(row["gas-price"])
                    txn["gas-price"] = row["gas-price"]
                except ValueError as e:
                    print(f"{Typgpy.FAIL}Error on line {i}: {e}{Typgpy.ENDC}")
                    print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                    continue
            if row["gas-limit"]:
                try:
                    int(row["gas-limit"])
                    txn["gas-limit"] = row["gas-limit"]
                except ValueError as e:
                    print(f"{Typgpy.FAIL}Error on line {i}: {e}{Typgpy.ENDC}")
                    print(f"{Typgpy.WARNING}Skipping!{Typgpy.ENDC}")
                    continue
            data.append(txn)
    print("\nFinished parsing CSV!")
    return data


def sanity_check(args):
    """
    Sanity check before starting the scrip to terminate early.
    """
    assert os.path.isfile(args.path), f"{args.path} is not a file"
    assert os.path.exists(args.path), f"{args.path} does not exist"
    try:
        return_code = urllib.request.urlopen(args.node, context=ssl._create_unverified_context()).getcode()
    except (urllib.error.HTTPError, urllib.error.URLError) as e:
        raise RuntimeError(f"unable to connect to node {args.node}") from e
    assert return_code == 200, f"bad response code ({return_code}) from node {args.node}"
    if args.chain_id is not None:
        assert args.chain_id in chain_id_options, f"{args.chain_id} not in {chain_id_options}"


def _setup_itc():
    """
    Setup `_itc_call_and_prefix` depending on if itc.sh exists.
    """
    global _itc_call_and_prefix
    _itc_call_and_prefix = [f"{script_directory}/itc"]
    try:
        _itc(["version"])
        return
    except (subprocess.CalledProcessError, FileNotFoundError):
        print(f"Unable to execute itc CLI directly at: '{_itc_call_and_prefix[0]}'")
        print(f"Trying to use 'itc.sh'...")
    if "itc.sh" in os.listdir(script_directory):
        _itc_call_and_prefix = [f"{script_directory}/itc.sh", "--"]
        try:
            _itc(["version"])
            return
        except subprocess.CalledProcessError as e:
            raise SystemExit(
                f"'itc.sh' is unable to execute the CLI. Try downloading the CLI with `./itc.sh -d`.") from e
    else:
        raise SystemExit(f"'itc.sh' is not found in script directory {script_directory}. ")


def _parse_args():
    """
    Argument parser that is only used for main execution.
    """
    parser = argparse.ArgumentParser(description='Intelchain CLI, transaction from CSV file wrapper script.')
    parser.add_argument("path", type=str, help="The path to the CSV file.")
    parser.add_argument("--node", "-n", dest="node", default="https://testnet.intelchain.network/", type=str,
                        help="The node or endpoint to send the transactions to, default: 'https://testnet.intelchain.network/'.")
    parser.add_argument("--batch-size", dest="batch_size", default=4, type=int,
                        help="Number of transactions to send in 1 batch to the CLI before checking output, default: 4")
    parser.add_argument("--timeout-per-tx", dest="timeout_per_tx", default=40, type=int,
                        help="Max time spent checking for a single transaction to confirm. Option is ignored "
                             "if --fast is enabled. Default 40.")
    parser.add_argument("--chain-id", dest="chain_id", default=None, type=str,
                        help="The chain ID of the transactions. Default uses implicit chain ID from CLI. "
                             f"Options: {chain_id_options}")
    parser.add_argument("--fast", action="store_true",
                        help="Send transactions without waiting for transaction confirmation.")
    parser.add_argument("--use-default-passphrase", action="store_true",
                        help="Use default passphrase if no passphrase file or string is provided in given CSV file.")
    parser.add_argument("--yes", action="store_true", help="Say yes to confirmation check")
    parser.add_argument("--verbose", action="store_true", help="Enable verbose mode when sending transactions")
    args = parser.parse_args()
    args.path = os.path.expanduser(args.path)
    return args


if __name__ == "__main__":
    args = _parse_args()
    _setup_itc()
    sanity_check(args)
    transactions = parse_csv(args.path, args.node, use_default_passphrase=args.use_default_passphrase)
    send_transactions(transactions, args.batch_size, args.node, args.chain_id,
                      timeout=args.timeout_per_tx, fast=args.fast, yes=args.yes)
